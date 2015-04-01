package osmtopo

import (
	"github.com/gogo/protobuf/proto"
	"github.com/omniscale/imposm3/element"
	"github.com/paulsmith/gogeos/geos"
	"github.com/rubenv/osmtopo"
	"github.com/rubenv/osmtopo/simplify"
)

func NodeFromEl(el element.Node) *Node {
	node := &Node{
		Id:  proto.Int64(el.Id),
		Lat: proto.Float64(el.Lat),
		Lon: proto.Float64(el.Long),
	}
	tags := []*TagEntry{}
	for k, v := range el.Tags {
		tags = append(tags, &TagEntry{
			Key:   proto.String(k),
			Value: proto.String(v),
		})
	}
	node.Tags = tags
	return node
}

func WayFromEl(el element.Way) *Way {
	way := &Way{
		Id:   proto.Int64(el.Id),
		Refs: el.Refs,
	}
	tags := []*TagEntry{}
	for k, v := range el.Tags {
		tags = append(tags, &TagEntry{
			Key:   proto.String(k),
			Value: proto.String(v),
		})
	}
	way.Tags = tags
	return way
}

func RelationFromEl(n element.Relation) *Relation {
	rel := &Relation{
		Id: proto.Int64(n.Id),
	}
	tags := []*TagEntry{}
	for k, v := range n.Tags {
		tags = append(tags, &TagEntry{
			Key:   proto.String(k),
			Value: proto.String(v),
		})
	}
	rel.Tags = tags
	members := []*MemberEntry{}
	for _, v := range n.Members {
		members = append(members, &MemberEntry{
			Id:   proto.Int64(v.Id),
			Type: proto.Int32(int32(v.Type)),
			Role: proto.String(v.Role),
		})
	}
	rel.Members = members
	return rel
}

func (r *Relation) GetTag(key string) (string, bool) {
	if r.Tags == nil {
		return "", false
	}

	for _, e := range r.Tags {
		if e.GetKey() == key {
			return e.GetValue(), true
		}
	}

	return "", false
}

func (r *Relation) ToGeometry(s *Store) (*geos.Geometry, error) {
	outerParts := [][]int64{}
	innerParts := [][]int64{}
	for _, m := range relation.GetMembers() {
		if m.GetType() == 1 && m.GetRole() == "outer" {
			way, err := store.GetWay(m.GetId())
			if err != nil {
				return err
			}

			outerParts = append(outerParts, way.GetRefs())
		}

		if m.GetType() == 1 && m.GetRole() == "inner" {
			way, err := store.GetWay(m.GetId())
			if err != nil {
				return err
			}

			innerParts = append(innerParts, way.GetRefs())
		}
	}

	outerParts = simplify.Reduce(outerParts)
	innerParts = simplify.Reduce(innerParts)

	outerPolys, err := toGeom(store, outerParts)
	if err != nil {
		return err
	}
	innerPolys, err := toGeom(store, innerParts)
	if err != nil {
		return err
	}

	polygons := make([]*geos.Geometry, 0)
	for _, shell := range outerPolys {
		holes := make([][]geos.Coord, 0)

		if len(innerPolys) > 0 {
			pshell := geos.PrepareGeometry(shell)

			// Find holes
			for i := 0; i < len(innerPolys); i++ {
				hole := innerPolys[i]
				c, err := pshell.Contains(hole)
				if err != nil {
					return err
				}
				if c {
					s, err := hole.Shell()
					if err != nil {
						return err
					}

					c, err := s.Coords()
					if err != nil {
						return err
					}

					holes = append(holes, c)
					innerPolys = append(innerPolys[:i], innerPolys[i+1:]...)
					i-- // Counter-act the increment at the end of the iteration
				}
			}
		}

		s, err := shell.Shell()
		if err != nil {
			return err
		}

		scoords, err := s.Coords()
		if err != nil {
			return err
		}

		polygon, err := geos.NewPolygon(scoords, holes...)
		if err != nil {
			return err
		}
		polygons = append(polygons, polygon)
	}

	var feat *geos.Geometry
	if len(polygons) == 1 {
		feat = polygons[0]
	} else {
		f, err := geos.NewCollection(geos.MULTIPOLYGON, polygons...)
		if err != nil {
			return err
		}
		feat = f
	}

	return feat, nil
}

func toGeom(store *osmtopo.Store, coords [][]int64) ([]*geos.Geometry, error) {
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

func expandPoly(store *osmtopo.Store, coords []int64) (*geos.Geometry, error) {
	points := make([]geos.Coord, len(coords))
	for i, c := range coords {
		node, err := store.GetNode(c)
		if err != nil {
			return nil, err
		}
		points[i] = geos.Coord{X: node.GetLon(), Y: node.GetLat()}
	}

	return geos.NewPolygon(points)
}
