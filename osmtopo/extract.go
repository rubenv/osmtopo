package osmtopo

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path"

	"github.com/paulsmith/gogeos/geos"
	"github.com/rubenv/osmtopo/geojson"
)

type ExtractConfig struct {
	Languages []string          `yaml:"languages"`
	Layers    map[string]*Layer `yaml:"layers"`
}

type Extractor struct {
	store   *Store
	config  *ExtractConfig
	outPath string
}

type Layer struct {
	Items []*ItemSelector `yaml:"items"`
}

type LayerOutput struct {
	Name       string
	Geometries []*LayerFeature
}

type LayerFeature struct {
	ID       int64
	Geometry *geos.Geometry
}

type ItemSelector struct {
	// Select by ID
	ID int64 `yaml:"id'`

	// Select by admin level and parent
	Parent     int64 `yaml:"parent'`
	AdminLevel int64 `yaml:"admin_level'`
}

func (e *Extractor) Run() error {
	if e.config.Layers == nil {
		return errors.New("No layers defined!")
	}

	err := os.MkdirAll(e.outPath, 0755)
	if err != nil {
		return err
	}

	// Load land geometries
	log.Println("Loading land geometries")
	keys, err := e.store.GetFeatures("land")
	if err != nil {
		return err
	}

	if len(keys) == 0 {
		return errors.New("No land found, did you forget to import first?")
	}

	clipGeos := make([]*geos.Geometry, 0, len(keys))
	for _, key := range keys {
		f, err := e.store.GetFeature("land", key)
		if err != nil {
			return err
		}

		feature := &geojson.Feature{}
		err = json.Unmarshal(f.GetGeojson(), feature)
		if err != nil {
			return err
		}

		geom, err := feature.ToGeos()
		if err != nil {
			return err
		}

		clipGeos = append(clipGeos, geom)
	}

	clipGeo, err := geos.NewCollection(geos.GEOMETRYCOLLECTION, clipGeos...)
	if err != nil {
		return err
	}

	for name, layer := range e.config.Layers {
		log.Printf("Processing layer %s", name)
		output, err := e.ProcessLayer(name, layer)
		if err != nil {
			return err
		}

		err = e.ClipLayer(clipGeo, output)
		if err != nil {
			return err
		}

		err = e.StoreOutput(output)
		if err != nil {
			return err
		}
	}

	return nil
}

func (e *Extractor) ProcessLayer(name string, layer *Layer) (*LayerOutput, error) {
	output := &LayerOutput{
		Name: name,
	}

	for _, item := range layer.Items {
		if item.ID > 0 {
			// ID-based selection
			relation, err := e.store.GetRelation(item.ID)
			if err != nil {
				return nil, err
			}
			if item == nil {
				return nil, fmt.Errorf("Unknown item ID: %d", item.ID)
			}

			geom, err := relation.ToGeometry(e.store)
			if err != nil {
				return nil, err
			}

			output.Geometries = append(output.Geometries, &LayerFeature{
				ID:       item.ID,
				Geometry: geom,
			})
		}
	}

	return output, nil
}

func (e *Extractor) ClipLayer(clipGeo *geos.Geometry, output *LayerOutput) error {
	// Clip each extracted geometry with the land geometries
	for _, feature := range output.Geometries {
		clipped, err := feature.Geometry.Intersection(clipGeo)
		// We ignore clipping errors here, these may happen when a self-intersection occurs
		if err == nil {
			feature.Geometry = clipped
		} else {
			log.Println(err)
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
		out, err := geojson.FromGeos(geom.Geometry)
		if err != nil {
			return err
		}

		out.Properties = map[string]string{
			"id": fmt.Sprintf("%d", geom.ID),
		}

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
