package osmtopo

import (
	"sync"

	"github.com/omniscale/imposm3/element"
	"github.com/omniscale/imposm3/parser/pbf"
)

type Import struct {
	Store *Store
	File  *pbf.Pbf

	wg  sync.WaitGroup
	err error

	nodes     chan []element.Node
	ways      chan []element.Way
	relations chan []element.Relation
}

func (i *Import) Run() error {
	i.nodes = make(chan []element.Node, 100)
	i.ways = make(chan []element.Way, 100)
	i.relations = make(chan []element.Relation, 100)

	i.wg.Add(3)

	go i.importNodes()
	go i.importWays()
	go i.importRelations()
	go i.startParser()

	i.wg.Wait()

	return i.err
}

func (i *Import) startParser() {
	parser := pbf.NewParser(i.File, i.nodes, i.nodes, i.ways, i.relations)
	parser.Start()
	parser.Close()

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
	}
}
