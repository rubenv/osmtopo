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

func (s *Store) addNewNode(n element.Node) error {
	//log.Printf("N: %#v\n", n)
	err := s.put(fmt.Sprintf("node/%d", n.Id), []byte("1"))
	if err != nil {
		return err
	}
	return nil
}

func (s *Store) addNewWay(w element.Way) error {
	//log.Printf("W: %#v\n", w)
	err := s.put(fmt.Sprintf("way/%d", w.Id), []byte("1"))
	if err != nil {
		return err
	}
	return nil
}

func (s *Store) addNewRelation(r element.Relation) error {
	//log.Printf("R: %#v\n", r)
	err := s.put(fmt.Sprintf("relation/%d", r.Id), []byte("1"))
	if err != nil {
		return err
	}
	return nil
}

func (s *Store) put(key string, value []byte) error {
	return s.db.Put(s.wo, []byte(key), value)
}
