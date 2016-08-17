package osmtopo

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"path"
	"strings"

	"github.com/cheggaaa/pb"
	"github.com/paulmach/go.geojson"
	"github.com/paulsmith/gogeos/geos"
	"github.com/rubenv/topojson"
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
	bar := pb.StartNew(len(keys))
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

		bar.Increment()
	}
	e.clipGeos = clipGeos
	bar.Finish()

	e.config.Layer.Output = "toplevel"
	return e.extractLayers([]*ConfigLayer{e.config.Layer}, 0)
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

	bar := pb.StartNew(geometries)
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
				return fmt.Errorf("Failed to convert to geometry: %s", err)
			}

			// TODO: Clip geometry if needed

			item.Geometry = geom

			bar.Increment()
		}

		err := e.ClipLayer(e.clipGeos, output)
		if err != nil {
			return err
		}
	}
	bar.Finish()

	// Build one big feature collection for simplification
	log.Printf("Simplifying\n")

	fc := geojson.NewFeatureCollection()
	for _, output := range outputs {
		for _, item := range output.Geometries {
			g, err := GeometryFromGeos(item.Geometry)
			if err != nil {
				return err
			}

			out := geojson.NewFeature(g)
			out.SetProperty("id", fmt.Sprintf("%d", item.ID))

			fc.AddFeature(out)

			// No longer needed, we still have the ID as a reference
			item.Geometry = nil
		}
	}

	// Build a topology for simplification
	maxErr := float64(0)
	if len(e.config.Simplify) > depth {
		maxErr = math.Pow(10, float64(-e.config.Simplify[depth]))
	}
	topo := topojson.NewTopology(fc, &topojson.TopologyOptions{
		PreQuantize:  0,
		PostQuantize: 1e5,
		Simplify:     maxErr,
		IDProperty:   "id",
	})

	log.Printf("Outputting\n")
	bar = pb.StartNew(len(outputs))
	for _, output := range outputs {
		err := e.StoreOutput(output, topo)
		if err != nil {
			return err
		}
		bar.Increment()
	}
	bar.Finish()

	// Free the outputs & topology
	outputs = nil
	topo = nil

	log.Printf("Processing at level %d: DONE\n", depth)

	// Process the child layers
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

func (e *Extractor) StoreOutput(output *LayerOutput, topo *topojson.Topology) error {
	if len(output.Geometries) == 0 {
		return nil
	}

	ids := make([]string, len(output.Geometries))
	for i, geom := range output.Geometries {
		ids[i] = fmt.Sprintf("%d", geom.ID)
	}

	// Filter topology
	topo = FilterTopology(topo, ids)
	if len(topo.Objects) == 0 {
		return nil
	}

	// Prepare output
	dir := path.Join(e.outPath, fmt.Sprintf("%d", output.Depth))
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return err
	}

	name := strings.Replace(output.Name, " ", "-", -1)
	name = strings.ToLower(name)

	// Write TopoJSON
	outFile, err := os.Create(path.Join(dir, fmt.Sprintf("%s.topojson", name)))
	if err != nil {
		return err
	}
	defer outFile.Close()

	err = json.NewEncoder(outFile).Encode(topo)
	if err != nil {
		return err
	}

	// Write GeoJSON
	fc := topo.ToGeoJSON()
	outFile2, err := os.Create(path.Join(dir, fmt.Sprintf("%s.geojson", name)))
	if err != nil {
		return err
	}
	defer outFile2.Close()

	err = json.NewEncoder(outFile2).Encode(fc)
	if err != nil {
		return err
	}

	return nil
}
