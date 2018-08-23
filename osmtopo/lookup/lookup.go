// Index structure for multi-layered indexing of spacial topologies.
//
// Or in easier terms: index a bunch of shapes, then ask: "in which shapes does this point fall?".
package lookup

import (
	"fmt"
	"strconv"

	"github.com/Workiva/go-datastructures/augmentedtree"
	"github.com/golang/geo/s2"
	geojson "github.com/paulmach/go.geojson"
	"github.com/rubenv/topojson"
)

type Data struct {
	layers map[string]*layer
}

type layer struct {
	tree     augmentedtree.Tree
	loops    map[int64]int64
	polygons map[int64]*loopPolygon
}

func New() *Data {
	return &Data{
		layers: make(map[string]*layer),
	}
}

// Index a geometry into a given level
//
// Note that concurrency is not supported! You should always index all data prior to doing any querying.
func (l *Data) IndexGeometry(layerID string, id int64, geom *geojson.Geometry) error {
	layer, ok := l.layers[layerID]
	if !ok {
		layer = newLayer()
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

// Index all geometries of a topology into a given level
//
// Note that concurrency is not supported! You should always index all data prior to doing any querying.
func (l *Data) IndexTopology(layerID string, topo *topojson.Topology) error {
	fc := topo.ToGeoJSON()
	for _, feat := range fc.Features {
		id, err := strconv.ParseInt(feat.ID.(string), 10, 64)
		if err != nil {
			return err
		}

		err = l.IndexGeometry(layerID, id, feat.Geometry)
		if err != nil {
			return fmt.Errorf("IndexGeometry: %s", err)
		}
	}

	return nil
}

func newLayer() *layer {
	return &layer{
		polygons: make(map[int64]*loopPolygon),
		loops:    make(map[int64]int64),
		tree:     augmentedtree.New(1),
	}
}

func (l *layer) indexPolygon(id int64, poly [][][]float64) error {
	rc := s2.RegionCoverer{
		MinLevel: 4,
		MaxLevel: 22,
		MaxCells: 8,
	}

	if len(poly[0]) <= 4 && hasDuplicates(poly[0]) {
		return nil
	}

	outer := makeLoop(poly[0])
	err := outer.Validate()
	if err != nil {
		return fmt.Errorf("Invalid outer loop for %d: %s", id, err)
	}

	inner := make([]*s2.Loop, 0)
	for i, coords := range poly[1:] {
		loop := makeLoop(coords)
		if loop == nil {
			continue
		}

		err := loop.Validate()
		if err != nil {
			return fmt.Errorf("Invalid inner loop %d for %d: %s", i, id, err)
		}
		inner = append(inner, loop)
	}

	loopId := int64(len(l.loops))
	l.loops[loopId] = id
	l.polygons[loopId] = &loopPolygon{
		outer: outer,
		inner: inner,
	}

	covering := rc.Covering(&region{outer})

	// Find a pre-existing interval
	for _, cell := range covering {
		ival := &interval{Cell: cell}
		results := l.tree.Query(ival)

		added := false
		for _, result := range results {
			i := result.(*interval)
			if ival.EqualAtDimension(result, 1) {
				i.Loops = append(i.Loops, loopId)
				added = true
			}
		}

		if !added {
			ival.Loops = []int64{loopId}
			l.tree.Add(ival)
		}
	}

	return nil
}

// Look up all shapes that contain a given point, in a given layer
func (l *Data) Query(lat, lng float64, layerID string) ([]int64, error) {
	layer, ok := l.layers[layerID]
	if !ok {
		return nil, nil
	}

	cell := s2.CellIDFromLatLng(s2.LatLngFromDegrees(lat, lng))
	ival := &interval{Cell: cell}

	matches := make([]int64, 0)
	results := layer.tree.Query(ival)
	for _, r := range results {
		result := r.(*interval)
		for _, loop := range result.Loops {
			geomId := layer.loops[loop]
			poly := layer.polygons[loop]

			if poly.IsInside(lat, lng) {
				matches = append(matches, geomId)
			}
		}
	}

	return matches, nil
}
