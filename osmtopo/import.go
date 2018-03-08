package osmtopo

import (
	"context"
	"fmt"
	"os"

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

	/*
		started time.Time
		running bool
		pwg     sync.WaitGroup
	*/

	coords    chan []element.Node
	nodes     chan []element.Node
	ways      chan []element.Way
	relations chan []element.Relation

	nodeCount     *atomic.Int64
	wayCount      *atomic.Int64
	relationCount *atomic.Int64

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
	}

}

func (i *importer) Run() error {
	_, err := os.Stat(i.filename)
	if err != nil {
		return err
	}

	// Pass 1: Import relations
	i.log("importing relations")
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
