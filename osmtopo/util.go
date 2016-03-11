package osmtopo

import (
	"fmt"

	"github.com/paulsmith/gogeos/geos"
	"github.com/rubenv/osmtopo/geojson"
)

func FeatureFromGeos(geom *geos.Geometry) (*geojson.Feature, error) {
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

		features := make([]*geojson.Feature, c)
		for i := 0; i < c; i++ {
			g, err := geom.Geometry(i)
			if err != nil {
				return nil, err
			}

			f, err := FeatureFromGeos(g)
			if err != nil {
				return nil, err
			}

			features[i] = f
		}

		fc := &geojson.Feature{
			Type:     "FeatureCollection",
			Features: features,
		}

		return fc, nil
	case geos.POLYGON:
		rings, err := polyToRings(geom)
		if err != nil {
			return nil, err
		}

		p := &geojson.Feature{
			Type: "Feature",
			Geometry: &geojson.Geometry{
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

		rings := make([][][]geojson.Coordinate, c)

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

		p := &geojson.Feature{
			Type: "Feature",
			Geometry: &geojson.Geometry{
				Type:        "MultiPolygon",
				Coordinates: rings,
			},
		}

		return p, nil
	default:
		return nil, fmt.Errorf("Unknown geometry type: %v", t)
	}
}

func polyToRings(geom *geos.Geometry) ([][]geojson.Coordinate, error) {
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

	rings := make([][]geojson.Coordinate, len(holes)+1)
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

func toCoordinates(ring *geos.Geometry) ([]geojson.Coordinate, error) {
	n, err := ring.NPoint()
	if err != nil {
		return nil, err
	}

	coords := make([]geojson.Coordinate, n)
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

		coords[i] = geojson.Coordinate{x, y}
	}
	return coords, nil
}

func FeatureToGeos(f *geojson.Feature) (*geos.Geometry, error) {
	switch f.Type {
	case "Feature":
		switch f.Geometry.Type {
		case "Polygon":
			coords, err := toCoordSlices(f.Geometry.Coordinates)
			if err != nil {
				return nil, err
			}
			shell := coords[0]
			holes := coords[1:]
			return geos.NewPolygon(shell, holes...)
		case "MultiPolygon":
			geoms := []*geos.Geometry{}
			if objs, ok := f.Geometry.Coordinates.([]interface{}); ok {
				for _, c := range objs {
					coords, err := toCoordSlices(c)
					if err != nil {
						return nil, err
					}
					shell := coords[0]
					holes := coords[1:]
					poly, err := geos.NewPolygon(shell, holes...)
					if err != nil {
						return nil, err
					}
					geoms = append(geoms, poly)
				}
			} else {
				return nil, fmt.Errorf("Bad coordinates: %v", f.Geometry.Coordinates)
			}

			return geos.NewCollection(geos.MULTIPOLYGON, geoms...)
		default:
			return nil, fmt.Errorf("Unknown geometry type: %v", f.Geometry.Type)
		}
	default:
		return nil, fmt.Errorf("Unknown feature type: %v", f.Type)
	}
}

func toCoordSlices(obj interface{}) ([][]geos.Coord, error) {
	var coords [][]geojson.Coordinate
	if c, ok := obj.([][]geojson.Coordinate); ok {
		coords = c
	} else if c, ok := obj.([]interface{}); ok {
		for _, obj := range c {
			if p, ok := obj.([]interface{}); ok {
				ls := []geojson.Coordinate{}
				for _, p2 := range p {
					if point, ok := p2.([]interface{}); ok {
						coord := geojson.Coordinate{
							point[0].(float64),
							point[1].(float64),
						}
						ls = append(ls, coord)
					} else {
						return nil, fmt.Errorf("Bad inner type: %#v\n", p2)
					}
				}
				coords = append(coords, ls)
			} else {
				return nil, fmt.Errorf("Cannot convert member: %#v\n", obj)
			}
		}
	} else {
		return nil, fmt.Errorf("Cannot convert: %#v\n", obj)
	}

	result := make([][]geos.Coord, 0, len(coords))
	for _, c := range coords {
		points := make([]geos.Coord, 0, len(c))
		for _, p := range c {
			points = append(points, geos.Coord{
				X: p[0],
				Y: p[1],
			})
		}
		result = append(result, points)
	}

	return result, nil
}

func toGeosCoord(coords []interface{}) []geos.Coord {
	points := make([]geos.Coord, 0, len(coords))
	for _, coord := range coords {
		c := coord.([]interface{})
		points = append(points, geos.Coord{
			X: c[0].(float64),
			Y: c[1].(float64),
		})
	}
	return points
}

func toGeosCoords(coords [][]interface{}) [][]geos.Coord {
	result := make([][]geos.Coord, 0, len(coords))
	for _, coord := range coords {
		result = append(result, toGeosCoord(coord))
	}
	return result
}
