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

	coords    chan []element.Node
	nodes     chan []element.Node
	ways      chan []element.Way
	relations chan []element.Relation
}

func (i *Import) Run() error {
	i.coords = make(chan []element.Node, 100)
	i.nodes = make(chan []element.Node, 100)
	i.ways = make(chan []element.Way, 100)
	i.relations = make(chan []element.Relation, 100)

	i.wg.Add(4)

	go i.importCoords()
	go i.importNodes()
	go i.importWays()
	go i.importRelations()
	go i.startParser()

	i.wg.Wait()

	return i.err
}

func (i *Import) startParser() {
	parser := pbf.NewParser(i.File, i.coords, i.nodes, i.ways, i.relations)
	parser.Start()
	parser.Close()

	close(i.coords)
	close(i.nodes)
	close(i.ways)
	close(i.relations)
}

func (i *Import) importCoords() {
	defer i.wg.Done()
	for {
		arr, ok := <-i.coords
		if !ok {
			return
		}
		if i.err != nil {
			continue
		}

		for _, n := range arr {
			err := i.Store.addNewNode(n)
			if err != nil {
				i.err = err
			}
		}
	}
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

		for _, n := range arr {
			err := i.Store.addNewNode(n)
			if err != nil {
				i.err = err
			}
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

		for _, n := range arr {
			err := i.Store.addNewWay(n)
			if err != nil {
				i.err = err
			}
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

		for _, n := range arr {
			err := i.Store.addNewRelation(n)
			if err != nil {
				i.err = err
			}
		}
	}
}
