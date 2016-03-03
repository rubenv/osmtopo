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
	err     error

	nodes     chan []element.Node
	ways      chan []element.Way
	relations chan []element.Relation

	nodeCount     int64
	wayCount      int64
	relationCount int64
}

func (i *Import) Run() error {
	i.nodes = make(chan []element.Node, 1000)
	i.ways = make(chan []element.Way, 1000)
	i.relations = make(chan []element.Relation, 1000)

	i.wg.Add(3)
	i.running = true
	i.started = time.Now()

	go i.importNodes()
	go i.importWays()
	go i.importRelations()
	go i.startParser()
	go i.updateProgress()

	i.wg.Wait()
	i.running = false

	return i.err
}

func (i *Import) updateProgress() {
	prevNodeCount := int64(0)
	prevWayCount := int64(0)
	prevRelationCount := int64(0)
	tick := time.Tick(1 * time.Second)

	for i.running {
		executing := time.Now().Sub(i.started)
		newNodes := i.nodeCount - prevNodeCount
		newWays := i.wayCount - prevWayCount
		newRelations := i.relationCount - prevRelationCount

		fmt.Printf("\r[N: %12d (%7d/s)] [W: %12d (%7d/s)] [R: %12d (%7d/s)] %s", i.nodeCount, newNodes, i.wayCount, newWays, i.relationCount, newRelations, executing)

		prevNodeCount += newNodes
		prevWayCount += newWays
		prevRelationCount += newRelations
		<-tick
	}

	fmt.Println()
}

func (i *Import) startParser() {
	parser := pbf.NewParser(i.File, i.nodes, i.nodes, i.ways, i.relations)
	parser.Parse()

	close(i.nodes)
	close(i.ways)
	close(i.relations)
}

func (i *Import) importNodes() {
	defer i.wg.Done()
	for {
		arr, ok := <-i.nodes
		if !ok {
			return
		}
		if i.err != nil {
			continue
		}

		nodes := []*Node{}
		for _, n := range arr {
			nodes = append(nodes, NodeFromEl(n))
		}
		err := i.Store.addNewNodes(nodes)
		if err != nil {
			i.err = err
		}
		i.nodeCount += int64(len(arr))
	}
}

func (i *Import) importWays() {
	defer i.wg.Done()
	for {
		arr, ok := <-i.ways
		if !ok {
			return
		}
		if i.err != nil {
			continue
		}

		ways := []*Way{}
		for _, n := range arr {
			ways = append(ways, WayFromEl(n))
		}
		err := i.Store.addNewWays(ways)
		if err != nil {
			i.err = err
		}
		i.wayCount += int64(len(arr))
	}
}

func (i *Import) importRelations() {
	defer i.wg.Done()
	for {
		arr, ok := <-i.relations
		if !ok {
			return
		}
		if i.err != nil {
			continue
		}

		rels := []*Relation{}
		for _, n := range arr {
			rels = append(rels, RelationFromEl(n))
		}
		err := i.Store.addNewRelations(rels)
		if err != nil {
			i.err = err
		}
		i.relationCount += int64(len(arr))
	}
}
