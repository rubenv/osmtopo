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
	geojson "github.com/paulmach/go.geojson"
	"github.com/rubenv/osmtopo/osmtopo/model"
	"github.com/rubenv/topojson"
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

	err = e.downloadWater(tmp, "water.zip")
	if err != nil {
		return err
	}

	filename := path.Join(tmp, "water.zip")
	err = e.importWater(filename, tmp)
	if err != nil {
		return err
	}

	e.waterLock.Lock()
	e.waterClipGeos = make(map[string][]*clipGeometry)
	e.waterLock.Unlock()

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
			return fmt.Errorf("Failed to process polygon %d: %s", id, err)
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
	return e.addNewGeometries("water", geometries)
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

func (e *Env) loadWaterClipGeos(maxErr float64) ([]*clipGeometry, error) {
	e.waterLock.Lock()
	defer e.waterLock.Unlock()

	key := fmt.Sprintf("%f", maxErr)
	cg, ok := e.waterClipGeos[key]
	if ok {
		return cg, nil
	}

	// Load water geometries
	keys, err := e.GetGeometries("water")
	if err != nil {
		return nil, err
	}

	if len(keys) == 0 {
		return nil, errors.New("No water found, did you forget to import first?")
	}

	fc := geojson.NewFeatureCollection()
	clipGeos := make([]*clipGeometry, 0, len(keys))
	for _, key := range keys {
		f, err := e.GetGeometry("water", key)
		if err != nil {
			return nil, err
		}

		geometry := &geojson.Geometry{}
		err = json.Unmarshal(f.Geojson, geometry)
		if err != nil {
			return nil, err
		}

		out := geojson.NewFeature(geometry)
		out.SetProperty("id", fmt.Sprintf("%d", key))

		fc.AddFeature(out)
	}

	topo := topojson.NewTopology(fc, &topojson.TopologyOptions{
		Simplify:   maxErr,
		IDProperty: "id",
	})
	fc = topo.ToGeoJSON()

	for _, feat := range fc.Features {
		geom, err := GeometryToGeos(feat.Geometry)
		if err != nil {
			return nil, err
		}

		geom, err = geom.Buffer(0)
		if err != nil {
			return nil, err
		}

		clipGeos = append(clipGeos, &clipGeometry{
			Geometry: geom,
			Prepared: geom.Prepare(),
		})
	}

	e.waterClipGeos[key] = clipGeos
	return clipGeos, nil
}
