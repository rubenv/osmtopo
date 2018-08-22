package osmtopo

import (
	"archive/zip"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"path"
	"strings"

	shp "github.com/jonas-p/go-shp"
	geojson "github.com/paulmach/go.geojson"
	"github.com/paulsmith/gogeos/geos"
)

type boundingBox []float64

func newBoundingBox() boundingBox {
	return boundingBox{
		math.MaxFloat64,
		math.MaxFloat64,
		-math.MaxFloat64,
		-math.MaxFloat64,
	}
}

func (b boundingBox) bound(p []float64) {
	x := p[0]
	y := p[1]

	if x < b[0] {
		b[0] = x
	}
	if x > b[2] {
		b[2] = x
	}
	if y < b[1] {
		b[1] = y
	}
	if y > b[3] {
		b[3] = y
	}
}

func (b boundingBox) boundPoints(l [][]float64) {
	for _, p := range l {
		b.bound(p)
	}
}

func (b boundingBox) boundMulti(ml [][][]float64) {
	for _, l := range ml {
		for _, p := range l {
			b.bound(p)
		}
	}
}

func (b boundingBox) extend(bb boundingBox) {
	b.bound([]float64{bb[0], bb[1]})
	b.bound([]float64{bb[2], bb[3]})
}

func GeometryFromGeos(geom *geos.Geometry) (*geojson.Geometry, error) {
	t, err := geom.Type()
	if err != nil {
		return nil, err
	}

	bb := newBoundingBox()

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
			bb.extend(f.BoundingBox)
		}

		gc := geojson.NewCollectionGeometry(geometries...)
		gc.BoundingBox = bb
		return gc, nil
	case geos.POLYGON:
		rings, err := polyToRings(geom)
		if err != nil {
			return nil, err
		}
		bb.boundMulti(rings)

		p := geojson.NewPolygonGeometry(rings)
		p.BoundingBox = bb
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
			bb.boundMulti(r)
		}

		p := geojson.NewMultiPolygonGeometry(rings...)
		p.BoundingBox = bb
		return p, nil
	default:
		return nil, fmt.Errorf("Unknown geometry type: %v", t)
	}
}

func polyToRings(geom *geos.Geometry) ([][][]float64, error) {
	shell, err := geom.Shell()
	if err != nil {
		return nil, fmt.Errorf("Failed to grab shell: %s", err)
	}
	c, err := toCoordinates(shell)
	if err != nil {
		return nil, err
	}

	holes, err := geom.Holes()
	if err != nil {
		return nil, fmt.Errorf("Failed to grab holes: %s", err)
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

func unpackFile(f *zip.File, folder string) error {
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

func missingKey(id string) []byte {
	return []byte(fmt.Sprintf("missing/%s", id))
}

func stampKey(stamp string) []byte {
	return []byte(fmt.Sprintf("stamp/%s", stamp))
}

func flagKey(flag string) []byte {
	return []byte(fmt.Sprintf("flag/%s", flag))
}

func intKey(nbr string) []byte {
	return []byte(fmt.Sprintf("int/%s", nbr))
}
