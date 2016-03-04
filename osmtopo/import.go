package osmtopo

import (
	"fmt"
	"sync"
	"time"

	"github.com/omniscale/imposm3/element"
	"github.com/omniscale/imposm3/parser/pbf"
)

type Import struct {
	Store *Store
	File  *pbf.Pbf

	started time.Time
	running bool
	wg      sync.WaitGroup
	pwg     sync.WaitGroup
	err     error

	coords    chan []element.Node
	nodes     chan []element.Node
	ways      chan []element.Way
	relations chan []element.Relation

	nodeCount     int64
	wayCount      int64
	relationCount int64
}

func (i *Import) Run() error {
	i.coords = make(chan []element.Node, 1000)
	i.nodes = make(chan []element.Node, 1000)
	i.ways = make(chan []element.Way, 1000)
	i.relations = make(chan []element.Relation, 1000)

	i.wg.Add(3)
	i.pwg.Add(1)
	i.running = true
	i.started = time.Now()

	go i.importNodes()
	go i.importWays()
	go i.importRelations()
	go i.startParser()
	go i.updateProgress()

	i.wg.Wait()
	i.running = false
	i.pwg.Wait()

	return i.err
}

func (i *Import) updateProgress() {
	defer i.pwg.Done()

	prevNodeCount := int64(0)
	prevWayCount := int64(0)
	prevRelationCount := int64(0)
	every := int64(10)
	tick := time.Tick(time.Duration(every) * time.Second)

	update := func() {
		executing := time.Now().Sub(i.started)
		newNodes := (i.nodeCount - prevNodeCount) / every
		newWays := (i.wayCount - prevWayCount) / every
		newRelations := (i.relationCount - prevRelationCount) / every

		elapsed := time.Duration(executing.Seconds()) * time.Second

		fmt.Printf("\r[N: %12d (%7d/s)] [W: %12d (%7d/s)] [R: %12d (%7d/s)] %s", i.nodeCount, newNodes, i.wayCount, newWays, i.relationCount, newRelations, elapsed)

		prevNodeCount += newNodes
		prevWayCount += newWays
		prevRelationCount += newRelations
	}

	for i.running {
		update()
		<-tick
	}

	update()

	fmt.Println()
}

func (i *Import) startParser() {
	parser := pbf.NewParser(i.File, i.coords, i.nodes, i.ways, i.relations)
	parser.Parse()
}

func (i *Import) importNodes() {
	defer i.wg.Done()
	nodeChan := i.nodes
	coordChan := i.coords
	el := []element.Node{}

	nodes := []*Node{}
	batchSize := 2500000

	for nodeChan != nil || coordChan != nil {
		select {
		case arr, ok := <-coordChan:
			if !ok {
				coordChan = nil
				continue
			}
			el = arr
		case arr, ok := <-nodeChan:
			if !ok {
				nodeChan = nil
				continue
			}
			el = arr
		}

		if i.err != nil {
			continue
		}

		for _, n := range el {
			nodes = append(nodes, NodeFromEl(n))
		}

		if len(nodes) > batchSize {
			err := i.Store.addNewNodes(nodes)
			if err != nil {
				i.err = err
			}
			i.nodeCount += int64(len(nodes))
			nodes = []*Node{}
		}
	}

	if len(nodes) > 0 {
		err := i.Store.addNewNodes(nodes)
		if err != nil {
			i.err = err
		}
		i.nodeCount += int64(len(nodes))
	}
}

func (i *Import) importWays() {
	defer i.wg.Done()

	ways := []*Way{}
	batchSize := 10000000

	for {
		arr, ok := <-i.ways
		if !ok {
			break
		}
		if i.err != nil {
			continue
		}

		for _, n := range arr {
			ways = append(ways, WayFromEl(n))
		}

		if len(ways) > batchSize {
			err := i.Store.addNewWays(ways)
			if err != nil {
				i.err = err
			}
			i.wayCount += int64(len(ways))
			ways = []*Way{}
		}
	}

	if len(ways) > 0 {
		err := i.Store.addNewWays(ways)
		if err != nil {
			i.err = err
		}
		i.wayCount += int64(len(ways))
	}
}

func (i *Import) importRelations() {
	defer i.wg.Done()

	rels := []*Relation{}
	batchSize := 10000

	for {
		arr, ok := <-i.relations
		if !ok {
			break
		}
		if i.err != nil {
			continue
		}

		for _, n := range arr {
			rels = append(rels, RelationFromEl(n))
		}

		if len(rels) > batchSize {
			err := i.Store.addNewRelations(rels)
			if err != nil {
				i.err = err
			}
			i.relationCount += int64(len(rels))
			rels = []*Relation{}
		}
	}

	if len(rels) > 0 {
		err := i.Store.addNewRelations(rels)
		if err != nil {
			i.err = err
		}
		i.relationCount += int64(len(rels))
	}
}
