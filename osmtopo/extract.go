package osmtopo

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"strings"

	"github.com/paulmach/go.geojson"
	"github.com/paulsmith/gogeos/geos"
)

type Extractor struct {
	store    *Store
	config   *Config
	outPath  string
	clipGeos []*ClipGeometry
}

type LayerOutput struct {
	Name       string
	Depth      int
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
	e.clipGeos = clipGeos

	e.config.Layer.Output = "toplevel"
	return e.extractLayers([]*ConfigLayer{e.config.Layer}, 0)
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

func (e *Extractor) extractLayers(layers []*ConfigLayer, depth int) error {
	if len(layers) == 0 {
		return nil
	}

	outputs := make(map[string]*LayerOutput)

	// Collect geometries
	geometries := 0
	for _, layer := range layers {
		if layer.ID == 0 {
			continue
		}

		output, ok := outputs[layer.Output]
		if !ok {
			output = &LayerOutput{
				Name:  layer.Output,
				Depth: depth,
			}
			outputs[layer.Output] = output
		}

		output.Geometries = append(output.Geometries, &LayerFeature{
			ID: layer.ID,
		})
		geometries++
	}

	log.Printf("Processing at level %d, %d geometries, %d outputs\n", depth, geometries, len(outputs))

	for _, output := range outputs {
		for _, item := range output.Geometries {
			relation, err := e.store.GetRelation(item.ID)
			if err != nil {
				return err
			}
			if item == nil {
				return fmt.Errorf("Unknown item ID: %d", item.ID)
			}

			geom, err := ToGeometry(relation, e.store)
			if err != nil {
				return err
			}

			item.Geometry = geom
		}

		err := e.ClipLayer(e.clipGeos, output)
		if err != nil {
			return err
		}
	}

	// TODO: Simplify

	for _, output := range outputs {
		err := e.StoreOutput(output)
		if err != nil {
			return err
		}
	}

	childLayers := make([]*ConfigLayer, 0)
	for _, layer := range layers {
		for _, child := range layer.Children {
			child.Output = layer.Name
			childLayers = append(childLayers, child)
		}
	}

	return e.extractLayers(childLayers, depth+1)
}

type ClipGeometry struct {
	Geometry *geos.Geometry
	Prepared *geos.PGeometry
}

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
	if len(output.Geometries) == 0 {
		return nil
	}

	dir := path.Join(e.outPath, fmt.Sprintf("%d", output.Depth))
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return err
	}

	fc := geojson.NewFeatureCollection()

	for _, geom := range output.Geometries {
		g, err := GeometryFromGeos(geom.Geometry)
		if err != nil {
			return err
		}

		out := geojson.NewFeature(g)
		out.SetProperty("id", fmt.Sprintf("%d", geom.ID))

		fc.AddFeature(out)

	}

	name := strings.Replace(output.Name, " ", "-", -1)
	name = strings.ToLower(name)
	outFile, err := os.Create(path.Join(dir, fmt.Sprintf("%s.geojson", name)))
	if err != nil {
		return err
	}

	err = json.NewEncoder(outFile).Encode(fc)
	outFile.Close()
	if err != nil {
		return err
	}

	return nil
}
