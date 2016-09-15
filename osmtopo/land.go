package osmtopo

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"os"
	"path"
	"reflect"
	"strconv"
	"strings"

	"github.com/jonas-p/go-shp"
	"github.com/paulmach/go.geojson"
	"github.com/rubenv/osmtopo/osmtopo/model"
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

	geometries := make([]*model.Geometry, 0)
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

		geometry, err := l.processPolygon(id, poly)
		if err != nil {
			return err
		}
		if geometry != nil {
			geometries = append(geometries, geometry)
		}
	}

	log.Println("Removing old geometries")
	err = l.store.removeGeometries("land")
	if err != nil {
		return err
	}

	log.Printf("Storing %d geometries", len(geometries))
	err = l.store.addNewGeometries("land", geometries)
	if err != nil {
		return err
	}

	log.Println("Done")

	return nil
}

func (l *Land) processPolygon(id int64, poly *shp.Polygon) (*model.Geometry, error) {
	outer := make([][]shp.Point, 0)
	inner := make([][]shp.Point, 0)

	for i, first := range poly.Parts {
		last := len(poly.Points)
		if i < len(poly.Parts)-1 {
			last = int(poly.Parts[i+1])
		}

		points := poly.Points[first:last]

		if len(points) < 3 {
			continue
		}

		/*
			// Simplify
			path := geo.NewPathPreallocate(len(points), len(points))
			for i, p := range points {
				path.SetAt(i, &geo.Point{p.X, p.Y})
			}
			simplified := reducers.VisvalingamThreshold(path, 1e-8)

			points = []shp.Point{}
			length := simplified.Length()
			for j := 0; j < length; j++ {
				point := simplified.GetAt(j)
				points = append(points, shp.Point{
					X: point[0],
					Y: point[1],
				})
			}
		*/

		// Drop tiny geometries
		area := ringArea(points)
		if math.Abs(area) < 1e-5 {
			continue
		}

		if area >= 0 {
			outer = append(outer, points)
		} else {
			// Holes are encoded counter-clockwise in
			// shape files, thus leading to a negative
			// area
			inner = append(inner, points)
		}
	}

	poly = nil

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

	outer = nil
	inner = nil

	feat, err := MakePolygons(outerPolys, innerPolys)
	if err != nil {
		return nil, err
	}

	outerPolys = nil
	innerPolys = nil

	out, err := GeometryFromGeos(feat)
	if err != nil {
		return nil, err
	}

	/*
		// Simplify
		fc := geojson.NewFeatureCollection()
		fc.AddFeature(geojson.NewFeature(out))

		topology := topojson.NewTopology(fc, &topojson.TopologyOptions{
			Simplify: math.Pow(10, -8),
		})

		// Back to GeoJSON
		fc = topology.ToGeoJSON()
		if len(fc.Features) == 0 {
			return nil, nil
		}
		out = fc.Features[0].Geometry

		if out.Type == geojson.GeometryPolygon && polyRingArea(out.Polygon[0]) < 1e-5 {
			return nil, nil
		}
	*/

	// Store it
	b, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}

	return &model.Geometry{
		Id:      id,
		Geojson: b,
	}, nil
}

func (l *Land) Export(filename string) error {
	keys, err := l.store.GetGeometries("land")
	if err != nil {
		return err
	}

	if len(keys) == 0 {
		return errors.New("No land found, did you forget to import first?")
	}

	geometries := make([]*geojson.Geometry, len(keys))
	for i, key := range keys {
		g, err := l.store.GetGeometry("land", key)
		if err != nil {
			return err
		}

		geometry := &geojson.Geometry{}
		err = json.Unmarshal(g.Geojson, geometry)
		if err != nil {
			return err
		}

		geometries[i] = geometry
	}

	result := geojson.NewCollectionGeometry(geometries...)

	out, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer out.Close()

	return json.NewEncoder(out).Encode(result)
}
