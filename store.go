package osmtopo

//go:generate protoc --gogo_out=. storage.proto

import (
	"fmt"

	"github.com/gogo/protobuf/proto"
	"github.com/jmhodges/levigo"
	"github.com/omniscale/imposm3/element"
	"github.com/omniscale/imposm3/parser/pbf"
)

type Store struct {
	path    string
	db      *levigo.DB
	indexer *Indexer

	wo *levigo.WriteOptions
	ro *levigo.ReadOptions
}

func NewStore(path string) (*Store, error) {
	store := &Store{
		path: path,
	}

	store.indexer = &Indexer{
		store: store,
	}

	opts := levigo.NewOptions()
	opts.SetCache(levigo.NewLRUCache(3 << 30))
	opts.SetCreateIfMissing(true)
	opts.SetFilterPolicy(levigo.NewBloomFilter(10))
	db, err := levigo.Open(path, opts)
	if err != nil {
		return nil, err
	}
	store.db = db

	store.wo = levigo.NewWriteOptions()
	store.ro = levigo.NewReadOptions()

	return store, nil
}

func (s *Store) Import(file string) error {
	f, err := pbf.Open(file)
	if err != nil {
		return err
	}

	i := Import{
		Store: s,
		File:  f,
	}
	return i.Run()
}

func (s *Store) addNewNodes(arr []element.Node) error {
	wb := levigo.NewWriteBatch()
	defer wb.Close()
	for _, n := range arr {
		node := &Node{
			Id:  proto.Int64(n.Id),
			Lat: proto.Float64(n.Lat),
			Lon: proto.Float64(n.Long),
		}
		tags := []*TagEntry{}
		for k, v := range n.Tags {
			tags = append(tags, &TagEntry{
				Key:   proto.String(k),
				Value: proto.String(v),
			})
		}
		node.Tags = tags
		data, err := proto.Marshal(node)
		if err != nil {
			return err
		}
		wb.Put([]byte(fmt.Sprintf("node/%d", n.Id)), data)
	}
	err := s.db.Write(s.wo, wb)
	if err != nil {
		return err
	}
	return nil
}

func (s *Store) addNewWays(arr []element.Way) error {
	wb := levigo.NewWriteBatch()
	defer wb.Close()
	for _, n := range arr {
		way := &Way{
			Id:   proto.Int64(n.Id),
			Refs: n.Refs,
		}
		tags := []*TagEntry{}
		for k, v := range n.Tags {
			tags = append(tags, &TagEntry{
				Key:   proto.String(k),
				Value: proto.String(v),
			})
		}
		way.Tags = tags
		data, err := proto.Marshal(way)
		if err != nil {
			return err
		}
		wb.Put([]byte(fmt.Sprintf("way/%d", n.Id)), data)
	}
	err := s.db.Write(s.wo, wb)
	if err != nil {
		return err
	}
	return nil
}

func (s *Store) addNewRelations(arr []element.Relation) error {
	wb := levigo.NewWriteBatch()
	defer wb.Close()
	for _, n := range arr {
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
		data, err := proto.Marshal(rel)
		if err != nil {
			return err
		}
		wb.Put([]byte(fmt.Sprintf("relation/%d", n.Id)), data)

		s.indexer.newRelation(n, wb)
	}

	err := s.db.Write(s.wo, wb)
	if err != nil {
		return err
	}
	return nil
}

func (s *Store) GetNode(id string) (*Node, error) {
	n, err := s.db.Get(s.ro, []byte(fmt.Sprintf("node/%s", id)))
	if err != nil {
		return nil, err
	}

	node := &Node{}
	err = proto.Unmarshal(n, node)
	if err != nil {
		return nil, err
	}

	return node, nil
}

func (s *Store) GetWay(id string) (*Way, error) {
	n, err := s.db.Get(s.ro, []byte(fmt.Sprintf("way/%s", id)))
	if err != nil {
		return nil, err
	}

	way := &Way{}
	err = proto.Unmarshal(n, way)
	if err != nil {
		return nil, err
	}

	return way, nil
}

func (s *Store) GetRelation(id string) (*Relation, error) {
	n, err := s.db.Get(s.ro, []byte(fmt.Sprintf("relation/%s", id)))
	if err != nil {
		return nil, err
	}

	rel := &Relation{}
	err = proto.Unmarshal(n, rel)
	if err != nil {
		return nil, err
	}

	return rel, nil
}
