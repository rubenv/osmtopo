package osmtopo

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/omniscale/imposm3/element"
	geojson "github.com/paulmach/go.geojson"
	"github.com/paulsmith/gogeos/geos"
	"github.com/rubenv/osmtopo/osmtopo/model"
	"github.com/rubenv/osmtopo/simplify"
)

func AcceptRelation(r model.Relation, blacklist []int64) bool {
	admin, _ := r.GetTag("admin_level")
	natural, _ := r.GetTag("natural")
	ok := admin != "" || natural == "water"
	if !ok {
		return false
	}

	for _, id := range blacklist {
		if r.Id == id {
			return false
		}
	}

	return true
}

func AcceptTag(k, v string) bool {
	if k == "admin_level" || k == "name" || strings.HasPrefix(k, "name:") {
		return true
	}
	return false
}

func NodeFromEl(el element.Node) model.Node {
	return model.Node{
		Id:  el.Id,
		Lat: el.Lat,
		Lon: el.Long,
	}
}

func WayFromEl(el element.Way) model.Way {
	return model.Way{
		Id:   el.Id,
		Refs: el.Refs,
	}
}

func RelationFromEl(n element.Relation) model.Relation {
	rel := model.Relation{
		Id: n.Id,
	}
	tags := []*model.TagEntry{}
	for k, v := range n.Tags {
		if !AcceptTag(k, v) {
			continue
		}
		tags = append(tags, &model.TagEntry{
			Key:   k,
			Value: v,
		})
	}
	rel.Tags = tags
	members := []*model.MemberEntry{}
	for _, v := range n.Members {
		members = append(members, &model.MemberEntry{
			Id:   v.Id,
			Type: int32(v.Type),
			Role: v.Role,
		})
	}
	rel.Members = members
	return rel
}

func ToGeometryCached(t string, r *model.Relation, e *Env) (*geojson.Geometry, error) {
	f, err := e.GetGeometry(t, r.Id)
	if err != nil {
		return nil, err
	}
	if f != nil {
		g := &geojson.Geometry{}
		err = json.Unmarshal(f.Geojson, g)
		if err != nil {
			return nil, err
		}
		return g, nil
	}

	g, err := ToGeometry(r, e)
	if err != nil {
		return nil, err
	}

	geom, err := GeometryFromGeos(g)
	if err != nil {
		return nil, fmt.Errorf("GeometryFromGeos: %s for relation %d: %#v", err, r.Id, g)
	}

	data, err := json.Marshal(geom)
	if err != nil {
		return nil, err
	}

	err = e.addGeometry(t, &model.Geometry{
		Id:      r.Id,
		Geojson: data,
	})
	if err != nil {
		return nil, err
	}

	return geom, nil
}

func ToGeometry(r *model.Relation, e *Env) (*geos.Geometry, error) {
	outerParts := [][]int64{}
	innerParts := [][]int64{}
	for _, m := range r.GetMembers() {
		if m.Type == 1 && m.Role == "outer" {
			way, err := e.GetWay(m.Id)
			if err != nil {
				return nil, err
			}
			if way == nil {
				//log.Printf("WARNING: Missing outer way %d for relation %d\n", m.Id, r.Id)
				continue
			}

			outerParts = append(outerParts, way.Refs)
		}

		if m.Type == 1 && m.Role == "inner" {
			way, err := e.GetWay(m.Id)
			if err != nil {
				return nil, err
			}
			if way == nil {
				//log.Printf("WARNING: Missing inner way %d for relation %d\n", m.Id, r.Id)
				continue
			}

			innerParts = append(innerParts, way.Refs)
		}
	}

	outerParts = simplify.Reduce(outerParts)
	innerParts = simplify.Reduce(innerParts)

	outerPolys, err := toGeom(e, outerParts)
	if err != nil {
		return nil, err
	}
	innerPolys, err := toGeom(e, innerParts)
	if err != nil {
		return nil, err
	}

	return MakePolygons(outerPolys, innerPolys)
}

func toGeom(env *Env, coords [][]int64) ([]*geos.Geometry, error) {
	linestrings := make([]*geos.Geometry, len(coords))
	for i, v := range coords {
		ls, err := expandPoly(env, v)
		if err != nil {
			return nil, err
		}
		linestrings[i] = ls
	}

	return linestrings, nil
}

func expandPoly(env *Env, coords []int64) (*geos.Geometry, error) {
	points := make([]geos.Coord, len(coords))
	for i, c := range coords {
		node, err := env.GetNode(c)
		if err != nil {
			return nil, err
		}
		if node == nil {
			return nil, fmt.Errorf("Missing node: %d", c)
		}
		points[i] = geos.Coord{X: node.Lon, Y: node.Lat}
	}

	return geos.NewPolygon(points)
}
