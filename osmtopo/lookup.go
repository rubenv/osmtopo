package osmtopo

import (
	"fmt"
	"sync"

	"github.com/Workiva/go-datastructures/augmentedtree"
	"github.com/golang/geo/s2"
	geojson "github.com/paulmach/go.geojson"
	"github.com/paulsmith/gogeos/geos"
)

type lookupData struct {
	env       *Env
	layers    map[string]*lookupLayer
	layerLock sync.Mutex
}

type lookupLayer struct {
	tree     augmentedtree.Tree
	loops    map[int64]int64
	polygons map[int64]*loopPolygon
}

func newLookupData(env *Env) *lookupData {
	return &lookupData{
		env:    env,
		layers: make(map[string]*lookupLayer),
	}
}

func (l *lookupData) IndexGeometry(layerID string, id int64, geom *geojson.Geometry) error {
	layer, ok := l.layers[layerID]
	if !ok {
		layer = newLookupLayer()
		l.layers[layerID] = layer
	}

	switch geom.Type {
	case geojson.GeometryPolygon:
		err := layer.indexPolygon(id, geom.Polygon)
		if err != nil {
			return err
		}
	case geojson.GeometryMultiPolygon:
		for _, poly := range geom.MultiPolygon {
			err := layer.indexPolygon(id, poly)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func newLookupLayer() *lookupLayer {
	return &lookupLayer{
		polygons: make(map[int64]*loopPolygon),
		loops:    make(map[int64]int64),
		tree:     augmentedtree.New(1),
	}
}

func (l *lookupLayer) indexPolygon(id int64, poly [][][]float64) error {
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

func (l *lookupData) query(lat, lng float64, layerID string) ([]int64, error) {
	l.layerLock.Lock()
	layer, ok := l.layers[layerID]
	l.layerLock.Unlock()
	if !ok {
		return nil, fmt.Errorf("Unknown layer: %s", layerID)
	}

	cell := s2.CellIDFromLatLng(s2.LatLngFromDegrees(lat, lng))
	interval := &Interval{Cell: cell}

	matches := make([]int64, 0)
	results := layer.tree.Query(interval)
	for _, r := range results {
		result := r.(*Interval)
		for _, loop := range result.Loops {
			geomId := layer.loops[loop]
			poly := layer.polygons[loop]

			if poly.IsInside(lat, lng) {
				rel, err := l.env.GetRelation(geomId)
				if err != nil {
					return nil, err
				}
				if rel == nil {
					return nil, fmt.Errorf("Unknown relation: %d", geomId)
				}

				g, err := ToGeometry(rel, l.env)
				if err != nil {
					// Broken geometry, skip!
					continue
				}

				p, err := geos.NewPoint(geos.NewCoord(lng, lat))
				if err != nil {
					return nil, err
				}
				c, err := p.Within(g)
				if err != nil {
					return nil, err
				}
				if c {
					matches = append(matches, geomId)
				}
			}
		}
	}

	return matches, nil
}
