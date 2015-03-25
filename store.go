package osmtopo

//go:generate protoc --gogo_out=. storage.proto

import (
	"fmt"
	"os"

	"github.com/gogo/protobuf/proto"
	"github.com/jmhodges/levigo"
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
	err := os.MkdirAll(path+"/ldb", 0755)
	if err != nil {
		return nil, err
	}

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
	db, err := levigo.Open(path+"/ldb", opts)
	if err != nil {
		return nil, err
	}
	store.db = db

	store.wo = levigo.NewWriteOptions()
	store.ro = levigo.NewReadOptions()

	return store, nil
}

func (s *Store) Close() {
	s.db.Close()
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

func (s *Store) ApplyChange(file string) error {
	u := Update{
		Store:    s,
		Filename: file,
	}

	return u.Run()
}

func (s *Store) Reindex() error {
	return s.indexer.reindex()
}

func (s *Store) addNewNodes(arr []*Node) error {
	wb := levigo.NewWriteBatch()
	defer wb.Close()
	for _, n := range arr {
		data, err := proto.Marshal(n)
		if err != nil {
			return err
		}
		wb.Put([]byte(fmt.Sprintf("node/%d", n.GetId())), data)
	}
	return s.db.Write(s.wo, wb)
}

func (s *Store) removeNode(n *Node) error {
	wb := levigo.NewWriteBatch()
	defer wb.Close()
	wb.Delete([]byte(fmt.Sprintf("node/%d", n.GetId())))
	return s.db.Write(s.wo, wb)
}

func (s *Store) addNewWays(arr []*Way) error {
	wb := levigo.NewWriteBatch()
	defer wb.Close()
	for _, n := range arr {
		data, err := proto.Marshal(n)
		if err != nil {
			return err
		}
		wb.Put([]byte(fmt.Sprintf("way/%d", n.GetId())), data)
	}
	return s.db.Write(s.wo, wb)
}

func (s *Store) removeWay(n *Way) error {
	wb := levigo.NewWriteBatch()
	defer wb.Close()
	wb.Delete([]byte(fmt.Sprintf("way/%d", n.GetId())))
	return s.db.Write(s.wo, wb)
}

func (s *Store) addNewRelations(arr []*Relation) error {
	wb := levigo.NewWriteBatch()
	defer wb.Close()
	for _, n := range arr {
		data, err := proto.Marshal(n)
		if err != nil {
			return err
		}
		wb.Put([]byte(fmt.Sprintf("relation/%d", n.GetId())), data)

		s.indexer.newRelation(n, wb)
	}
	return s.db.Write(s.wo, wb)
}

func (s *Store) removeRelation(n *Relation) error {
	wb := levigo.NewWriteBatch()
	defer wb.Close()
	s.indexer.removeRelation(n, wb)
	wb.Delete([]byte(fmt.Sprintf("relation/%d", n.GetId())))
	return s.db.Write(s.wo, wb)
}

func (s *Store) GetNode(id string) (*Node, error) {
	n, err := s.db.Get(s.ro, []byte(fmt.Sprintf("node/%s", id)))
	if err != nil {
		return nil, err
	}

	if len(n) == 0 {
		return nil, nil
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

	if len(n) == 0 {
		return nil, nil
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

	if len(n) == 0 {
		return nil, nil
	}

	rel := &Relation{}
	err = proto.Unmarshal(n, rel)
	if err != nil {
		return nil, err
	}

	return rel, nil
}
