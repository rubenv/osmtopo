package osmtopo

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path"

	geo "github.com/paulmach/go.geo"
	geojson "github.com/paulmach/go.geojson"
	"github.com/rubenv/osmtopo/osmtopo/model"
	"github.com/rubenv/topojson"
)

func (e *Env) export() error {
	e.initialized.Wait()

	err := os.MkdirAll(e.outputPath, 0755)
	if err != nil {
		return err
	}

	for _, layer := range e.config.Layers {
		ids := e.topoData.Get(layer.ID)
		contains := make(map[int64]bool)
		for _, id := range ids {
			contains[id] = true
		}

		err := os.MkdirAll(path.Join(e.outputPath, layer.ID), 0755)
		if err != nil {
			return err
		}

		pipe := NewGeometryPipeline(e).
			Filter(func(rel *model.Relation) bool {
				return contains[rel.Id]
			}).
			Simplify(layer.Simplify).
			ClipWater().
			Quantize(1e6)

		topo, err := pipe.Run()
		if err != nil {
			return err
		}

		centers := make(map[string][]float64)
		for _, obj := range topo.Objects {
			bb := obj.BoundingBox
			centers[obj.ID] = []float64{
				bb[0] + bb[2]/2,
				bb[1] + bb[3]/2,
			}
		}

		slice := 0
		for len(centers) > 0 {
			aggCenter := []float64{0, 0}
			centerCount := 1

			toSelect := []string{}
			pointCount := 0

			// Split into a number of files of approximately the same size (based on point count)
			for pointCount < e.config.ExportPointLimit && len(centers) > 0 {
				currCenter := geo.NewPointFromLatLng(aggCenter[1]/float64(centerCount), aggCenter[0]/float64(centerCount))

				// Find closest topology
				closestDist := math.MaxFloat64
				key := ""
				for k, center := range centers {
					dist := currCenter.GeoDistanceFrom(geo.NewPointFromLatLng(center[1], center[0]))
					if dist < closestDist {
						key = k
						closestDist = dist
					}
				}

				// Update the aggregated center
				center := centers[key]
				aggCenter[0] += center[0]
				aggCenter[1] += center[1]
				centerCount += 1
				delete(centers, key)

				// Add it to the selection
				toSelect = append(toSelect, key)
				pointCount += countPoints(topo, topo.Objects[key])
			}

			fp, err := os.Create(path.Join(e.outputPath, layer.ID, fmt.Sprintf("%d.topojson", slice)))
			if err != nil {
				return err
			}

			filtered := topo.Filter(toSelect)
			err = json.NewEncoder(fp).Encode(filtered)
			if err != nil {
				fp.Close()
				return err
			}

			fp.Close()
			slice += 1
		}
	}
	return nil
}

func countPoints(topo *topojson.Topology, obj *topojson.Geometry) int {
	switch obj.Type {
	case geojson.GeometryPoint:
		return 1
	case geojson.GeometryMultiPoint:
		return len(obj.MultiPoint)
	case geojson.GeometryLineString:
		return countArcs(topo, obj.LineString)
	case geojson.GeometryMultiLineString:
		return countMultiArcs(topo, obj.MultiLineString)
	case geojson.GeometryPolygon:
		return countMultiArcs(topo, obj.Polygon)
	case geojson.GeometryMultiPolygon:
		result := 0
		for _, poly := range obj.MultiPolygon {
			result += countMultiArcs(topo, poly)
		}
		return result
	case geojson.GeometryCollection:
		result := 0
		for _, geometry := range obj.Geometries {
			result += countPoints(topo, geometry)
		}
		return result
	}
	return 0
}

func countMultiArcs(topo *topojson.Topology, marcs [][]int) int {
	result := 0
	for _, arcs := range marcs {
		for _, arc := range arcs {
			result += countArc(topo, arc)
		}
	}
	return result
}

func countArcs(topo *topojson.Topology, arcs []int) int {
	result := 0
	for _, arc := range arcs {
		result += countArc(topo, arc)
	}
	return result
}

func countArc(topo *topojson.Topology, arc int) int {
	if arc < 0 {
		arc = ^arc
	}
	return len(topo.Arcs[arc])
}
