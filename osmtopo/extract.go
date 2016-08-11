package osmtopo

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path"

	"github.com/paulmach/go.geojson"
	"github.com/paulsmith/gogeos/geos"
)

type Extractor struct {
	store   *Store
	config  *Config
	outPath string
}

type LayerOutput struct {
	Name       string
	Geometries []*LayerFeature
}

type LayerFeature struct {
	ID       int64
	Geometry *geos.Geometry
}

func (e *Extractor) Run() error {
	if e.config.Layer == nil {
		return errors.New("No layers defined!")
	}

	err := os.MkdirAll(e.outPath, 0755)
	if err != nil {
		return err
	}

	// Load water geometries
	log.Println("Loading water geometries")
	keys, err := e.store.GetGeometries("water")
	if err != nil {
		return err
	}

	if len(keys) == 0 {
		return errors.New("No water found, did you forget to import first?")
	}

	clipGeos := make([]*ClipGeometry, 0, len(keys))
	for _, key := range keys {
		f, err := e.store.GetGeometry("water", key)
		if err != nil {
			return err
		}

		geometry := &geojson.Geometry{}
		err = json.Unmarshal(f.Geojson, geometry)
		if err != nil {
			return err
		}

		geom, err := GeometryToGeos(geometry)
		if err != nil {
			return err
		}

		clipGeos = append(clipGeos, &ClipGeometry{
			Geometry: geom,
			Prepared: geom.Prepare(),
		})
	}

	/*
		// TODO
		for name, layer := range e.config.Layers {
			log.Printf("Processing layer %s", name)
			output, err := e.ProcessLayer(name, layer)
			if err != nil {
				return err
			}

			err = e.ClipLayer(clipGeos, output)
			if err != nil {
				return err
			}

			err = e.StoreOutput(output)
			if err != nil {
				return err
			}
		}
	*/

	return nil
}

type ClipGeometry struct {
	Geometry *geos.Geometry
	Prepared *geos.PGeometry
}

/*
// TODO
func (e *Extractor) ProcessLayer(name string, layer *Layer) (*LayerOutput, error) {
	output := &LayerOutput{
		Name: name,
	}

	for _, item := range layer.Items {
		if item.ID == 0 {
			return nil, fmt.Errorf("ID missing for item: %v", item)
		}

		relation, err := e.store.GetRelation(item.ID)
		if err != nil {
			return nil, err
		}
		if item == nil {
			return nil, fmt.Errorf("Unknown item ID: %d", item.ID)
		}

		geom, err := ToGeometry(relation, e.store)
		if err != nil {
			return nil, err
		}

		// TODO: Clip
		fmt.Printf("%#v\n", item.Clip)

		output.Geometries = append(output.Geometries, &LayerFeature{
			ID:       item.ID,
			Geometry: geom,
		})
	}

	return output, nil
}
*/

func (e *Extractor) ClipLayer(clipGeos []*ClipGeometry, output *LayerOutput) error {
	// Clip each extracted geometry with the water geometries
	for _, feature := range output.Geometries {
		for _, clipGeom := range clipGeos {
			intersects, err := clipGeom.Prepared.Intersects(feature.Geometry)
			if err != nil {
				return err
			}

			if intersects {
				clipped, err := feature.Geometry.Difference(clipGeom.Geometry)
				// We ignore clipping errors here, these may happen when a self-intersection occurs
				if err == nil {
					feature.Geometry = clipped
				} else {
					log.Println(err)
				}
			}
		}
	}

	return nil
}

func (e *Extractor) StoreOutput(output *LayerOutput) error {
	dir := path.Join(e.outPath, output.Name)
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return err
	}

	for _, geom := range output.Geometries {
		g, err := GeometryFromGeos(geom.Geometry)
		if err != nil {
			return err
		}

		out := geojson.NewFeature(g)
		out.SetProperty("id", fmt.Sprintf("%d", geom.ID))

		outFile, err := os.Create(path.Join(dir, fmt.Sprintf("%d.geojson", geom.ID)))
		if err != nil {
			return err
		}

		err = json.NewEncoder(outFile).Encode(out)
		outFile.Close()
		if err != nil {
			return err
		}
	}

	return nil
}
