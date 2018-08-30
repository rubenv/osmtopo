// Index structure for multi-layered indexing of spacial topologies.
//
// Or in easier terms: index a bunch of shapes, then ask: "in which shapes does this point fall?".
package lookup

import (
	"errors"
	"fmt"
	"strconv"
	"sync"

	"github.com/golang/geo/s2"
	geojson "github.com/paulmach/go.geojson"
	"github.com/rubenv/osmtopo/osmtopo/lookup/segtree"
)

type Data struct {
	layers    map[string]*layer
	layerLock sync.Mutex
	built     bool
}

type layer struct {
	indexLock sync.Mutex
	tree      *segtree.Tree
	loops     map[int64]int64
	//polygons map[int64]*loopPolygon
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
	switch geom.Type {
	case geojson.GeometryPolygon:
		err := l.IndexPolygon(layerID, id, geom.Polygon)
		if err != nil {
			return err
		}
	case geojson.GeometryMultiPolygon:
		for _, poly := range geom.MultiPolygon {
			err := l.IndexPolygon(layerID, id, poly)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (l *Data) IndexPolygon(layerID string, id int64, poly [][][]float64) error {
	covering, err := MakeCells(poly)
	if err != nil {
		return fmt.Errorf("%s for rel %d", err, id)
	}
	if covering == nil {
		return nil
	}

	return l.IndexCells(layerID, id, covering)
}

func (l *Data) IndexFeature(layerID string, feat *geojson.Feature) error {
	id := int64(0)
	switch v := feat.ID.(type) {
	case string:
		i, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return err
		}
		id = i
	case int64:
		id = v
	default:
		return fmt.Errorf("Unsupported ID type: %T", feat.ID)
	}

	err := l.IndexGeometry(layerID, id, feat.Geometry)
	if err != nil {
		return fmt.Errorf("IndexGeometry: %s", err)
	}
	return nil
}

func (l *Data) IndexCells(layerID string, id int64, cells s2.CellUnion) error {
	if l.built {
		return errors.New("Cannot index after building the lookup")
	}

	l.layerLock.Lock()
	layer, ok := l.layers[layerID]
	if !ok {
		layer = newLayer()
		l.layers[layerID] = layer
	}
	l.layerLock.Unlock()

	layer.indexCells(id, cells)
	return nil
}

// Index all geometries of a topology into a given level
//
// Note that concurrency is not supported! You should always index all data prior to doing any querying.
func (l *Data) IndexFeatures(layerID string, fc *geojson.FeatureCollection) error {
	for _, feat := range fc.Features {
		err := l.IndexFeature(layerID, feat)
		if err != nil {
			return err
		}
	}

	return nil
}

func (l *Data) Build() error {
	if l.built {
		return errors.New("Already built!")
	}

	l.layerLock.Lock()
	defer l.layerLock.Unlock()

	for _, layer := range l.layers {
		err := layer.tree.BuildTree()
		if err != nil {
			return err
		}
	}
	l.built = true
	return nil
}

func newLayer() *layer {
	return &layer{
		tree: &segtree.Tree{},
	}
}

func (l *layer) indexCells(id int64, cells s2.CellUnion) {
	l.indexLock.Lock()
	for _, cell := range cells {
		l.tree.Push(uint64(cell.RangeMin()), uint64(cell.RangeMax()), id)
	}
	l.indexLock.Unlock()
}

// Look up all shapes that contain a given point, in a given layer
func (l *Data) Query(lat, lng float64, layerID string) ([]int64, error) {
	layer, ok := l.layers[layerID]
	if !ok {
		return nil, nil
	}

	cell := s2.CellIDFromLatLng(s2.LatLngFromDegrees(lat, lng))

	matches := make([]int64, 0)
	results, err := layer.tree.QueryIndex(uint64(cell))
	if err != nil {
		return nil, err
	}
	for r := range results {
		matches = append(matches, r.(int64))
	}

	return matches, nil
}

func MakeCells(poly [][][]float64) (s2.CellUnion, error) {
	cov := s2.RegionCoverer{
		MinLevel: 4,
		MaxLevel: 22,
		MaxCells: 8,
	}

	if uniqueLength(poly[0]) < 4 {
		return nil, nil
	}

	outer := makeLoop(poly[0])
	if outer == nil {
		return nil, nil
	}

	err := outer.Validate()
	if err != nil {
		return nil, fmt.Errorf("Invalid outer loop: %s", err)
	}

	covering := cov.Covering(&region{outer})
	return covering, nil
}

func GeometryToCoverage(geom *geojson.Geometry) ([]s2.CellUnion, error) {
	switch geom.Type {
	case geojson.GeometryPolygon:
		cu, err := MakeCells(geom.Polygon)
		if err != nil {
			return nil, err
		}
		return []s2.CellUnion{cu}, nil
	case geojson.GeometryMultiPolygon:
		result := make([]s2.CellUnion, 0)
		for _, poly := range geom.MultiPolygon {
			cu, err := MakeCells(poly)
			if err != nil {
				return nil, err
			}
			result = append(result, cu)
		}
		return result, nil
	default:
		return nil, fmt.Errorf("Unsupported geometry: %s", geom.Type)
	}
}
