package osmtopo

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/omniscale/imposm3/element"
	"github.com/omniscale/imposm3/parser/pbf"
	"github.com/rubenv/osmtopo/osmtopo/model"
	"github.com/rubenv/osmtopo/osmtopo/needidx"
	"github.com/uber-go/atomic"
)

type importer struct {
	ctx      context.Context
	env      *Env
	name     string
	filename string

	started time.Time
	pwg     sync.WaitGroup

	coords    chan []element.Node
	nodes     chan []element.Node
	ways      chan []element.Way
	relations chan []element.Relation
	progress  chan interface{}

	nodeCount     *atomic.Int64
	wayCount      *atomic.Int64
	relationCount *atomic.Int64
	phase         *atomic.String

	nodesNeeded *needidx.NeedIdx
	waysNeeded  *needidx.NeedIdx
}

func newImporter(env *Env, name, filename string) *importer {
	return &importer{
		env:           env,
		name:          name,
		filename:      filename,
		nodesNeeded:   needidx.New(),
		waysNeeded:    needidx.New(),
		nodeCount:     atomic.NewInt64(0),
		wayCount:      atomic.NewInt64(0),
		relationCount: atomic.NewInt64(0),
		phase:         atomic.NewString(""),
		progress:      make(chan interface{}),
	}

}

func (i *importer) Run() error {
	_, err := os.Stat(i.filename)
	if err != nil {
		return err
	}

	i.started = time.Now()
	i.pwg.Add(1)
	go i.updateProgress()

	// Pass 1: Import relations
	i.phase.Store("relations")
	i.prepareChannels()
	g, ctx := errgroup.WithContext(i.env.ctx)
	i.ctx = ctx
	g.Go(i.discardNodes)
	g.Go(i.discardWays)
	g.Go(i.importRelations)
	g.Go(i.startParser)
	err = g.Wait()
	if err != nil {
		return err
	}

	// Pass 2: Import ways
	i.phase.Store("ways")
	i.prepareChannels()
	g, ctx = errgroup.WithContext(i.env.ctx)
	i.ctx = ctx
	g.Go(i.discardNodes)
	g.Go(i.importWays)
	g.Go(i.discardRelations)
	g.Go(i.startParser)
	err = g.Wait()
	if err != nil {
		return err
	}

	// Pass 2: Import nodes
	i.phase.Store("nodes")
	i.prepareChannels()
	g, ctx = errgroup.WithContext(i.env.ctx)
	i.ctx = ctx
	g.Go(i.importNodes)
	g.Go(i.discardWays)
	g.Go(i.discardRelations)
	g.Go(i.startParser)
	err = g.Wait()
	if err != nil {
		return err
	}

	close(i.progress)
	i.pwg.Wait()

	seconds := int64(time.Now().Sub(i.started).Seconds())
	if seconds == 0 {
		seconds = 1
	}
	elapsed := time.Duration(seconds) * time.Second
	i.log("[N: %d (%d/s)] [W: %d (%d/s)] [R: %d (%d/s)] %s", i.nodeCount.Load(), i.nodeCount.Load()/seconds, i.wayCount.Load(), i.wayCount.Load()/seconds, i.relationCount.Load(), i.relationCount.Load()/seconds, elapsed)

	return nil
}

func (i *importer) log(str string, args ...interface{}) {
	i.env.log(fmt.Sprintf("import/%s", i.name), str, args...)
}

func (i *importer) prepareChannels() {
	i.coords = make(chan []element.Node, 1000)
	i.nodes = make(chan []element.Node, 1000)
	i.ways = make(chan []element.Way, 1000)
	i.relations = make(chan []element.Relation, 1000)
}

func (i *importer) startParser() error {
	parser, err := pbf.NewParser(i.filename)
	if err != nil {
		return err
	}

	parser.Parse(i.coords, i.nodes, i.ways, i.relations)

	close(i.coords)
	close(i.nodes)
	close(i.ways)
	close(i.relations)

	return nil
}

func (i *importer) updateProgress() {
	defer i.pwg.Done()

	prevNodeCount := int64(0)
	prevWayCount := int64(0)
	prevRelationCount := int64(0)
	every := int64(1)

loop:
	for {
		select {
		case _, ok := <-i.progress:
			if !ok {
				break loop
			}
		case <-time.After(time.Duration(every) * time.Second):
		}

		executing := time.Now().Sub(i.started)
		newNodes := i.nodeCount.Load() - prevNodeCount
		newWays := i.wayCount.Load() - prevWayCount
		newRelations := i.relationCount.Load() - prevRelationCount

		elapsed := time.Duration(executing.Seconds()) * time.Second

		fmt.Printf("\r\033[K[N: %12d (%7d/s)] [W: %12d (%7d/s)] [R: %12d (%7d/s)] %s (%s)", i.nodeCount.Load(), newNodes/every, i.wayCount.Load(), newWays/every, i.relationCount.Load(), newRelations/every, elapsed, i.phase.Load())

		prevNodeCount += newNodes
		prevWayCount += newWays
		prevRelationCount += newRelations
	}
	fmt.Printf("\r\033[K")
}

func (i *importer) discardNodes() error {
	nodeChan := i.nodes
	coordChan := i.coords
	for nodeChan != nil || coordChan != nil {
		select {
		case _, ok := <-coordChan:
			if !ok {
				coordChan = nil
				continue
			}
		case _, ok := <-nodeChan:
			if !ok {
				nodeChan = nil
				continue
			}
		}
	}
	return nil
}

func (i *importer) discardWays() error {
	for {
		if _, ok := <-i.ways; !ok {
			return nil
		}
	}
}

func (i *importer) discardRelations() error {
	for {
		if _, ok := <-i.relations; !ok {
			return nil
		}
	}
}

func (i *importer) importNodes() error {
	nodeChan := i.nodes
	coordChan := i.coords

	nodes := []model.Node{}
	batchSize := 2500000

	done := i.ctx.Done()
loop:
	for nodeChan != nil || coordChan != nil {
		var arr []element.Node
		select {
		case a, ok := <-coordChan:
			if !ok {
				coordChan = nil
				continue
			}
			arr = a
		case a, ok := <-nodeChan:
			if !ok {
				nodeChan = nil
				continue
			}
			arr = a
		case <-done:
			break loop
		}

		for _, n := range arr {
			if !i.nodesNeeded.IsNeeded(n.Id) {
				continue
			}
			nodes = append(nodes, NodeFromEl(n))
		}

		if len(nodes) > batchSize {
			err := i.env.addNewNodes(nodes)
			if err != nil {
				return err
			}
			i.nodeCount.Add(int64(len(nodes)))
			nodes = []model.Node{}
		}
	}

	if len(nodes) > 0 {
		err := i.env.addNewNodes(nodes)
		if err != nil {
			return err
		}
		i.nodeCount.Add(int64(len(nodes)))
	}

	return nil
}

func (i *importer) importWays() error {
	ways := []model.Way{}
	batchSize := 100000

	done := i.ctx.Done()
loop:
	for {
		var arr []element.Way
		select {
		case a, ok := <-i.ways:
			if !ok {
				break loop
			}
			arr = a
		case <-done:
			break loop
		}

		for _, n := range arr {
			if !i.waysNeeded.IsNeeded(n.Id) {
				continue
			}

			for _, r := range n.Refs {
				i.nodesNeeded.MarkNeeded(r)
			}

			ways = append(ways, WayFromEl(n))
		}

		if len(ways) > batchSize {
			err := i.env.addNewWays(ways)
			if err != nil {
				return err
			}
			i.wayCount.Add(int64(len(ways)))
			ways = []model.Way{}
		}
	}

	if len(ways) > 0 {
		err := i.env.addNewWays(ways)
		if err != nil {
			return err
		}
		i.wayCount.Add(int64(len(ways)))
	}

	return nil
}

func (i *importer) importRelations() error {
	rels := []model.Relation{}
	batchSize := 10000

	done := i.ctx.Done()
loop:
	for {
		var arr []element.Relation
		select {
		case a, ok := <-i.relations:
			if !ok {
				break loop
			}
			arr = a
		case <-done:
			break loop
		}

		for _, n := range arr {
			for _, v := range n.Members {
				if v.Type == element.WAY {
					i.waysNeeded.MarkNeeded(v.Id)
				}
			}

			r := RelationFromEl(n)
			admin, _ := r.GetTag("admin_level")
			natural, _ := r.GetTag("natural")
			accepted := admin != "" || natural == "water"
			if !accepted {
				continue
			}

			rels = append(rels, r)
		}

		if len(rels) > batchSize {
			err := i.env.addNewRelations(rels)
			if err != nil {
				return err
			}
			i.relationCount.Add(int64(len(rels)))
			rels = []model.Relation{}
		}
	}

	if len(rels) > 0 {
		err := i.env.addNewRelations(rels)
		if err != nil {
			return err
		}
		i.relationCount.Add(int64(len(rels)))
	}

	return nil
}
