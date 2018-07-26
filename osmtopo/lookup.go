package osmtopo

import (
	"sync"

	"github.com/Workiva/go-datastructures/augmentedtree"
	"github.com/golang/geo/s2"
	geojson "github.com/paulmach/go.geojson"
)

type lookupData struct {
	levels    map[int]*lookupLevel
	levelLock sync.Mutex
}

type lookupLevel struct {
	/*
		datafiles map[int64]string
		datasets  map[int64]int64
	*/
	tree     augmentedtree.Tree
	loops    map[int64]int64
	polygons map[int64]*loopPolygon
}

func newLookupData() *lookupData {
	return &lookupData{
		levels: make(map[int]*lookupLevel),
	}
}

func (l *lookupData) HasLevel(id int) bool {
	// Note: not locking here, we know it's safe
	_, ok := l.levels[id]
	return ok
}

func (l *lookupData) IndexGeometry(levelId int, id int64, geom *geojson.Geometry) error {
	level, ok := l.levels[levelId]
	if !ok {
		level = newLookupLevel()
		l.levels[levelId] = level
	}

	switch geom.Type {
	case geojson.GeometryPolygon:
		err := level.indexPolygon(id, geom.Polygon)
		if err != nil {
			return err
		}
	case geojson.GeometryMultiPolygon:
		for _, poly := range geom.MultiPolygon {
			err := level.indexPolygon(id, poly)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func newLookupLevel() *lookupLevel {
	return &lookupLevel{
		polygons: make(map[int64]*loopPolygon),
		loops:    make(map[int64]int64),
		tree:     augmentedtree.New(1),
	}
}

func (l *lookupLevel) indexPolygon(id int64, poly [][][]float64) error {
	rc := s2.RegionCoverer{
		MinLevel: 2,
		MaxLevel: 30,
		MaxCells: 8,
	}

	if len(poly[0]) <= 4 && hasDuplicates(poly[0]) {
		return nil
	}

	outer := makeLoop(poly[0])

	inner := make([]*s2.Loop, 0)
	for _, coords := range poly[1:] {
		inner = append(inner, makeLoop(coords))
	}

	loopId := int64(len(l.loops))
	l.loops[loopId] = id
	l.polygons[loopId] = &loopPolygon{
		outer: outer,
		inner: inner,
	}

	covering := rc.Covering(&Region{outer})

	// Find a pre-existing interval
	for _, cell := range covering {
		interval := &Interval{Cell: cell}
		results := l.tree.Query(interval)

		added := false
		for _, result := range results {
			i := result.(*Interval)
			if cell == i.Cell {
				i.Loops = append(i.Loops, loopId)
				added = true
			}
		}

		if !added {
			interval.Loops = []int64{loopId}
			l.tree.Add(interval)
		}
	}

	return nil
}
