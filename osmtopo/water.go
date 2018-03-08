package osmtopo

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"path"
	"reflect"
	"strconv"
	"strings"
	"time"

	shp "github.com/jonas-p/go-shp"
	"github.com/northbright/ctx/ctxdownload"
	geo "github.com/paulmach/go.geo"
	"github.com/paulmach/go.geo/reducers"
	"github.com/rubenv/osmtopo/osmtopo/model"
)

func (e *Env) updateWater() error {
	shouldRun, err := e.shouldRun("water", e.config.UpdateWaterEvery)
	if err != nil {
		return err
	}
	if !shouldRun {
		return nil
	}

	tmp, err := ioutil.TempDir("", "water")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	e.log("water", "tmp: %s", tmp)

	err = e.downloadWater(tmp, "water.zip")
	if err != nil {
		return err
	}

	filename := path.Join(tmp, "water.zip")
	err = e.importWater(filename, tmp)
	if err != nil {
		return err
	}

	e.log("water", "Done")
	return e.setTimestamp("water", time.Now())
}

func (e *Env) downloadWater(folder, filename string) error {
	e.log("water", "Downloading water data")
	buf := make([]byte, 2*1024*1024)
	url := "http://data.openstreetmapdata.com/water-polygons-split-4326.zip"
	_, err := ctxdownload.Download(e.ctx, url, folder, filename, buf, 3600)
	return err
}

func (e *Env) importWater(filename, folder string) error {
	e.log("water", "Unpacking water data")
	r, err := zip.OpenReader(filename)
	if err != nil {
		return err
	}
	defer r.Close()

	shpName := ""
	for _, f := range r.File {
		err = unpackFile(f, folder)
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
	if e.ctx.Err() != nil {
		return e.ctx.Err()
	}

	e.log("water", "Processing geometries")
	shape, err := shp.Open(path.Join(folder, shpName))
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

		geometry, err := e.processWaterPolygon(id, poly)
		if err != nil {
			return err
		}
		if geometry != nil {
			geometries = append(geometries, geometry)
		}
	}
	if e.ctx.Err() != nil {
		return e.ctx.Err()
	}

	e.log("water", "Removing old geometries")
	err = e.removeGeometries("water")
	if err != nil {
		return err
	}
	if e.ctx.Err() != nil {
		return e.ctx.Err()
	}

	e.log("water", "Storing %d geometries", len(geometries))
	err = e.addNewGeometries("water", geometries)
	if err != nil {
		return err
	}

	return nil
}

func (e *Env) processWaterPolygon(id int64, poly *shp.Polygon) (*model.Geometry, error) {
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

		// Simplify
		path := geo.NewPathPreallocate(len(points), len(points))
		for i, p := range points {
			path.SetAt(i, &geo.Point{p.X, p.Y})
		}
		simplified := reducers.VisvalingamThreshold(path, 1e-5)

		points = []shp.Point{}
		length := simplified.Length()
		for j := 0; j < length; j++ {
			point := simplified.GetAt(j)
			points = append(points, shp.Point{
				X: point[0],
				Y: point[1],
			})
		}

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

	// Apply a buffer to avoid self-intersections
	feat, err = feat.Buffer(0)
	if err != nil {
		return nil, err
	}

	out, err := GeometryFromGeos(feat)
	if err != nil {
		return nil, err
	}

	b, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}

	return &model.Geometry{
		Id:      id,
		Geojson: b,
	}, nil
}
