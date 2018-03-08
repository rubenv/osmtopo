package osmtopo

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/cheggaaa/pb"
	"github.com/paulmach/go.geojson"
	"github.com/rubenv/topojson"
)

type Water struct {
	store *Store
}

func (l *Water) Export(filename string) error {
	keys, err := l.store.GetGeometries("water")
	if err != nil {
		return err
	}

	if len(keys) == 0 {
		return errors.New("No water found, did you forget to import first?")
	}

	geometries := make([]*geojson.Geometry, len(keys))
	for i, key := range keys {
		g, err := l.store.GetGeometry("water", key)
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

func loadWaterClipGeos(maxErr float64, store *Store, progress bool) ([]*ClipGeometry, error) {
	// Load water geometries
	keys, err := store.GetGeometries("water")
	if err != nil {
		return nil, err
	}

	if len(keys) == 0 {
		return nil, errors.New("No water found, did you forget to import first?")
	}

	var bar *pb.ProgressBar
	if progress {
		bar = pb.StartNew(len(keys))
		defer bar.Finish()
	}

	fc := geojson.NewFeatureCollection()
	clipGeos := make([]*ClipGeometry, 0, len(keys))
	for _, key := range keys {
		f, err := store.GetGeometry("water", key)
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
		if progress {
			bar.Increment()
		}
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

		clipGeos = append(clipGeos, &ClipGeometry{
			Geometry: geom,
			Prepared: geom.Prepare(),
		})
	}

	return clipGeos, nil
}
