package osmtopo

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"reflect"
	"strconv"
	"strings"

	"github.com/gogo/protobuf/proto"
	"github.com/jonas-p/go-shp"
	"github.com/paulsmith/gogeos/geos"
	"github.com/rubenv/osmtopo/geojson"
)

type Land struct {
	store *Store
}

func (l *Land) Import(zipfile string) error {
	tmp, err := ioutil.TempDir("", "land")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	r, err := zip.OpenReader(zipfile)
	if err != nil {
		return err
	}
	defer r.Close()

	shpName := ""
	for _, f := range r.File {
		err = unpackFile(f, tmp)
		if err != nil {
			return err
		}

		if strings.HasSuffix(f.Name, ".shp") {
			parts := strings.Split(f.Name, "/")
			shpName = parts[len(parts)-1]
		}
	}

	if shpName == "" {
		return errors.New("No shape file found in zip")
	}
	log.Printf("Parsing %s", shpName)

	shape, err := shp.Open(path.Join(tmp, shpName))
	if err != nil {
		return err
	}
	defer shape.Close()

	features := make([]*Feature, 0)
	for shape.Next() {
		n, p := shape.Shape()
		poly, ok := p.(*shp.Polygon)
		if !ok {
			return fmt.Errorf("Non-polygon found: %s, %v", reflect.TypeOf(p).Elem(), p.BBox())
		}

		val := shape.ReadAttribute(n, 0)
		id, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return err
		}

		feature, err := l.processPolygon(id, poly)
		if err != nil {
			return err
		}
		if poly != nil {
			features = append(features, feature)
		}
	}

	err = l.store.addNewFeatures(features)
	if err != nil {
		return err
	}

	return nil
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

func (l *Land) processPolygon(id int64, poly *shp.Polygon) (*Feature, error) {
	totalArea := float64(0)

	outer := make([][]shp.Point, 0)
	inner := make([][]shp.Point, 0)

	for i, first := range poly.Parts {
		last := len(poly.Points)
		if i < len(poly.Parts)-1 {
			last = int(poly.Parts[i]) + 1
		}

		points := poly.Points[first:last]
		area := ringArea(points)
		if area < 1e-5 {
			continue
		}

		if area > 0 {
			outer = append(outer, points)
		} else {
			// Holes are encoded counter-clockwise in
			// shape files, thus leading to a negative
			// area
			inner = append(inner, points)
		}

		totalArea += area
	}

	if len(outer) == 0 {
		return nil, nil
	}

	outerPolys, err := shpToGeom(outer)
	if err != nil {
		return nil, err
	}
	innerPolys, err := shpToGeom(inner)
	if err != nil {
		return nil, err
	}

	feat, err := MakePolygons(outerPolys, innerPolys)
	if err != nil {
		return nil, err
	}

	// TODO: Simplify!

	out, err := geojson.FromGeos(feat)
	if err != nil {
		return nil, err
	}

	b, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}

	return &Feature{
		Id:      proto.Int64(id),
		Geojson: b,
	}, nil
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