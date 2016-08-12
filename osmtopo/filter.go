package osmtopo

import (
	geojson "github.com/paulmach/go.geojson"
	"github.com/rubenv/topojson"
)

func FilterTopology(topo *topojson.Topology, ids []string) *topojson.Topology {
	result := &topojson.Topology{
		Type:        topo.Type,
		Transform:   topo.Transform,
		BoundingBox: topo.BoundingBox,
	}

	arcMap := make(map[int]int)

	for _, g := range topo.Objects {
		geom := remapGeometry(arcMap, ids, g)
		if geom != nil {
			result.Objects = append(result.Objects, geom)
		}
	}

	result.Arcs = make([][][]float64, len(arcMap))
	for k, v := range arcMap {
		result.Arcs[v] = topo.Arcs[k]
	}

	return result
}
func remapLineString(arcMap map[int]int, in []int) []int {
	out := make([]int, len(in))

	for i, arc := range in {
		a := arc
		reverse := false
		if a < 0 {
			a = ^a
			reverse = true
		}

		idx, ok := arcMap[a]
		if !ok {
			idx = len(arcMap)
			arcMap[a] = idx
		}
		if reverse {
			out[i] = ^idx
		} else {
			out[i] = idx
		}
	}

	return out
}

func remapMultiLineString(arcMap map[int]int, in [][]int) [][]int {
	lines := make([][]int, len(in))
	for i, line := range in {
		lines[i] = remapLineString(arcMap, line)
	}
	return lines
}

func remapGeometry(arcMap map[int]int, ids []string, g *topojson.Geometry) *topojson.Geometry {
	found := false
	for _, id := range ids {
		if g.ID == id {
			found = true
			break
		}
	}
	if !found {
		return nil
	}

	geom := &topojson.Geometry{
		ID:         g.ID,
		Type:       g.Type,
		Properties: g.Properties,
	}

	switch g.Type {
	case geojson.GeometryPoint:
		geom.Point = g.Point
	case geojson.GeometryMultiPoint:
		geom.MultiPoint = g.MultiPoint
	case geojson.GeometryLineString:
		geom.LineString = remapLineString(arcMap, g.LineString)
	case geojson.GeometryMultiLineString:
		geom.MultiLineString = remapMultiLineString(arcMap, g.MultiLineString)
	case geojson.GeometryPolygon:
		geom.Polygon = remapMultiLineString(arcMap, g.Polygon)
	case geojson.GeometryMultiPolygon:
		polygons := make([][][]int, len(g.MultiPolygon))
		for i, poly := range g.MultiPolygon {
			polygons[i] = remapMultiLineString(arcMap, poly)
		}
		geom.MultiPolygon = polygons
	case geojson.GeometryCollection:
		geometries := make([]*topojson.Geometry, 0)
		for _, geometry := range g.Geometries {
			out := remapGeometry(arcMap, ids, geometry)
			if out != nil {
				geometries = append(geometries, out)
			}
		}
		geom.Geometries = geometries
	}

	return geom
}
