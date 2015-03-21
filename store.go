package osmtopo

import (
	"fmt"

	"github.com/jmhodges/levigo"
	"github.com/omniscale/imposm3/element"
	"github.com/omniscale/imposm3/parser/pbf"
)

type Store struct {
	path string
	db   *levigo.DB

	wo *levigo.WriteOptions
}

func NewStore(path string) (*Store, error) {
	store := &Store{
		path: path,
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
		wb.Put([]byte(fmt.Sprintf("node/%d", n.Id)), []byte("1"))
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
		wb.Put([]byte(fmt.Sprintf("way/%d", n.Id)), []byte("1"))
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
		wb.Put([]byte(fmt.Sprintf("relation/%d", n.Id)), []byte("1"))
	}
	err := s.db.Write(s.wo, wb)
	if err != nil {
		return err
	}
	return nil
}

// TODO: Store actual data with protobufs
// TODO: Add indexing hooks for relations
