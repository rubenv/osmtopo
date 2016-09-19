package osmtopo

import (
	"fmt"
	"log"
	"strings"

	"github.com/omniscale/imposm3/element"
	"github.com/paulsmith/gogeos/geos"
	"github.com/rubenv/osmtopo/osmtopo/model"
	"github.com/rubenv/osmtopo/simplify"
)

func AcceptTag(k, v string) bool {
	if k == "admin_level" || k == "name" || strings.HasPrefix(k, "name:") {
		return true
	}
	return false
}

func NodeFromEl(el element.Node) *model.Node {
	node := &model.Node{
		Id:  el.Id,
		Lat: el.Lat,
		Lon: el.Long,
	}
	tags := []*model.TagEntry{}
	for k, v := range el.Tags {
		if !AcceptTag(k, v) {
			continue
		}
		tags = append(tags, &model.TagEntry{
			Key:   k,
			Value: v,
		})
	}
	node.Tags = tags
	return node
}

func WayFromEl(el element.Way) *model.Way {
	way := &model.Way{
		Id:   el.Id,
		Refs: el.Refs,
	}
	tags := []*model.TagEntry{}
	for k, v := range el.Tags {
		if !AcceptTag(k, v) {
			continue
		}
		tags = append(tags, &model.TagEntry{
			Key:   k,
			Value: v,
		})
	}
	way.Tags = tags
	return way
}

func RelationFromEl(n element.Relation) *model.Relation {
	rel := &model.Relation{
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

func ToGeometry(r *model.Relation, s *Store) (*geos.Geometry, error) {
	outerParts := [][]int64{}
	innerParts := [][]int64{}
	for _, m := range r.GetMembers() {
		if m.Type == 1 && m.Role == "outer" {
			way, err := s.GetWay(m.Id)
			if err != nil {
				return nil, err
			}
			if way == nil {
				log.Printf("WARNING: Missing way %d for relation %d\n", m.Id, r.Id)
				continue
			}

			outerParts = append(outerParts, way.Refs)
		}

		if m.Type == 1 && m.Role == "inner" {
			way, err := s.GetWay(m.Id)
			if err != nil {
				return nil, err
			}

			innerParts = append(innerParts, way.Refs)
		}
	}

	outerParts = simplify.Reduce(outerParts)
	innerParts = simplify.Reduce(innerParts)

	outerPolys, err := toGeom(s, outerParts)
	if err != nil {
		return nil, err
	}
	innerPolys, err := toGeom(s, innerParts)
	if err != nil {
		return nil, err
	}

	return MakePolygons(outerPolys, innerPolys)
}

func toGeom(store *Store, coords [][]int64) ([]*geos.Geometry, error) {
	linestrings := make([]*geos.Geometry, len(coords))
	for i, v := range coords {
		ls, err := expandPoly(store, v)
		if err != nil {
			return nil, err
		}
		linestrings[i] = ls
	}

	return linestrings, nil
}

func expandPoly(store *Store, coords []int64) (*geos.Geometry, error) {
	points := make([]geos.Coord, len(coords))
	for i, c := range coords {
		node, err := store.GetNode(c)
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
