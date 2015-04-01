package geojson

import (
	"fmt"

	"github.com/paulsmith/gogeos/geos"
)

type Feature struct {
	Features   []*Feature        `json:"features,omitempty"`
	Geometry   *Geometry         `json:"geometry,omitempty"`
	Id         *int64            `json:"id,omitempty"`
	Properties map[string]string `json:"properties,omitempty"`
	Type       string            `json:"type"`
}

type Geometry struct {
	Type        string      `json:"type"`
	Coordinates interface{} `json:"coordinates"`
}

type Coordinate [2]float64

func FromGeos(geom *geos.Geometry) (*Feature, error) {
	t, err := geom.Type()
	if err != nil {
		return nil, err
	}

	switch t {
	case geos.GEOMETRYCOLLECTION:
		c, err := geom.NGeometry()
		if err != nil {
			return nil, err
		}

		features := make([]*Feature, c)
		for i := 0; i < c; i++ {
			g, err := geom.Geometry(i)
			if err != nil {
				return nil, err
			}

			f, err := FromGeos(g)
			if err != nil {
				return nil, err
			}

			features[i] = f
		}

		fc := &Feature{
			Type:     "FeatureCollection",
			Features: features,
		}

		return fc, nil
	case geos.POLYGON:
		rings, err := polyToRings(geom)
		if err != nil {
			return nil, err
		}

		p := &Feature{
			Type: "Feature",
			Geometry: &Geometry{
				Type:        "Polygon",
				Coordinates: rings,
			},
		}

		return p, nil
	case geos.MULTIPOLYGON:
		c, err := geom.NGeometry()
		if err != nil {
			return nil, err
		}

		rings := make([][][]Coordinate, c)

		for i := 0; i < c; i++ {
			g, err := geom.Geometry(i)
			if err != nil {
				return nil, err
			}

			r, err := polyToRings(g)
			if err != nil {
				return nil, err
			}

			rings[i] = r
		}

		p := &Feature{
			Type: "Feature",
			Geometry: &Geometry{
				Type:        "MultiPolygon",
				Coordinates: rings,
			},
		}

		return p, nil
	default:
		return nil, fmt.Errorf("Unknown geometry type: %v", t)
	}
}

func polyToRings(geom *geos.Geometry) ([][]Coordinate, error) {
	shell, err := geom.Shell()
	if err != nil {
		return nil, err
	}
	c, err := toCoordinates(shell)
	if err != nil {
		return nil, err
	}

	holes, err := geom.Holes()
	if err != nil {
		return nil, err
	}

	rings := make([][]Coordinate, len(holes)+1)
	rings[0] = c
	for i, h := range holes {
		c, err := toCoordinates(h)
		if err != nil {
			return nil, err
		}
		rings[i+1] = c
	}

	return rings, nil
}

func toCoordinates(ring *geos.Geometry) ([]Coordinate, error) {
	n, err := ring.NPoint()
	if err != nil {
		return nil, err
	}

	coords := make([]Coordinate, n)
	for i := 0; i < n; i++ {
		p, err := ring.Point(i)
		if err != nil {
			return nil, err
		}

		x, err := p.X()
		if err != nil {
			return nil, err
		}

		y, err := p.Y()
		if err != nil {
			return nil, err
		}

		coords[i] = Coordinate{x, y}
	}
	return coords, nil
}
