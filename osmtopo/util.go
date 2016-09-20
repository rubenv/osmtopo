package osmtopo

import (
	"archive/zip"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"strings"

	shp "github.com/jonas-p/go-shp"
	"github.com/paulmach/go.geojson"
	"github.com/paulsmith/gogeos/geos"
)

func GeometryFromGeos(geom *geos.Geometry) (*geojson.Geometry, error) {
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

		geometries := make([]*geojson.Geometry, c)
		for i := 0; i < c; i++ {
			g, err := geom.Geometry(i)
			if err != nil {
				return nil, err
			}

			f, err := GeometryFromGeos(g)
			if err != nil {
				return nil, err
			}

			geometries[i] = f
		}

		gc := geojson.NewCollectionGeometry(geometries...)
		return gc, nil
	case geos.POLYGON:
		rings, err := polyToRings(geom)
		if err != nil {
			return nil, err
		}

		p := geojson.NewPolygonGeometry(rings)
		return p, nil
	case geos.MULTIPOLYGON:
		c, err := geom.NGeometry()
		if err != nil {
			return nil, err
		}

		rings := make([][][][]float64, c)

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

		p := geojson.NewMultiPolygonGeometry(rings...)
		return p, nil
	default:
		return nil, fmt.Errorf("Unknown geometry type: %v", t)
	}
}

func polyToRings(geom *geos.Geometry) ([][][]float64, error) {
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

	rings := make([][][]float64, len(holes)+1)
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

func toCoordinates(ring *geos.Geometry) ([][]float64, error) {
	n, err := ring.NPoint()
	if err != nil {
		return nil, err
	}

	coords := make([][]float64, n)
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

		coords[i] = []float64{x, y}
	}
	return coords, nil
}

func GeometryToGeos(g *geojson.Geometry) (*geos.Geometry, error) {
	switch g.Type {
	case geojson.GeometryPolygon:
		coords, err := toCoordSlices(g.Polygon)
		if err != nil {
			return nil, err
		}
		shell := coords[0]
		holes := coords[1:]
		return geos.NewPolygon(shell, holes...)
	case geojson.GeometryMultiPolygon:
		geoms := []*geos.Geometry{}
		for _, c := range g.MultiPolygon {
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

		return geos.NewCollection(geos.MULTIPOLYGON, geoms...)
	default:
		return nil, fmt.Errorf("Unknown geometry type: %v", g.Type)
	}
}

func toCoordSlices(coords [][][]float64) ([][]geos.Coord, error) {
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

func ringArea(points []shp.Point) float64 {
	result := float64(0)
	length := len(points)
	for i := 0; i < length; i++ {
		next := (i + 1) % length

		p1 := points[i]
		p2 := points[next]

		result += (p2.X - p1.X) * (p2.Y + p1.Y)
	}

	return result / 2
}

func shpToGeom(coords [][]shp.Point) ([]*geos.Geometry, error) {
	linestrings := make([]*geos.Geometry, len(coords))
	for i, v := range coords {
		points := make([]geos.Coord, len(v))
		for j, c := range v {
			points[j] = geos.Coord{X: c.X, Y: c.Y}
		}
		ls, err := geos.NewPolygon(points)
		if err != nil {
			return nil, err
		}
		linestrings[i] = ls
	}

	return linestrings, nil
}

func unpackFile(f *zip.File, folder string) error {
	log.Printf("Unpacking %s\n", f.Name)

	parts := strings.Split(f.Name, "/")
	name := parts[len(parts)-1]

	out, err := os.Create(path.Join(folder, name))
	if err != nil {
		return err
	}
	defer out.Close()

	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	_, err = io.Copy(out, rc)
	return err
}

func nodeKey(id int64) []byte {
	buf := make([]byte, 13)
	copy(buf, "node/")
	binary.BigEndian.PutUint64(buf[5:], uint64(id))
	return buf
}

func wayKey(id int64) []byte {
	buf := make([]byte, 12)
	copy(buf, "way/")
	binary.BigEndian.PutUint64(buf[4:], uint64(id))
	return buf
}

func relationKey(id int64) []byte {
	buf := make([]byte, 17)
	copy(buf, "relation/")
	binary.BigEndian.PutUint64(buf[9:], uint64(id))
	return buf
}
