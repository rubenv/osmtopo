package osmtopo

import (
	"github.com/gogo/protobuf/proto"
	"github.com/omniscale/imposm3/element"
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
