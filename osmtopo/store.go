package osmtopo

//go:generate protoc --gogo_out=. storage.proto

import (
	"fmt"
	"io/ioutil"
	"os"

	"gopkg.in/yaml.v1"

	"github.com/gogo/protobuf/proto"
	"github.com/omniscale/imposm3/parser/pbf"
	"github.com/tecbot/gorocksdb"
)

type Store struct {
	path    string
	db      *gorocksdb.DB
	indexer *Indexer

	wo *gorocksdb.WriteOptions
	ro *gorocksdb.ReadOptions
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

	opts := gorocksdb.NewDefaultOptions()
	bb := gorocksdb.NewDefaultBlockBasedTableOptions()
	bb.SetBlockCache(gorocksdb.NewLRUCache(3 << 30))
	bb.SetFilterPolicy(gorocksdb.NewBloomFilter(10))
	opts.SetCreateIfMissing(true)
	opts.SetBlockBasedTableFactory(bb)
	db, err := gorocksdb.OpenDb(opts, path+"/ldb")
	if err != nil {
		return nil, err
	}
	store.db = db

	store.wo = gorocksdb.NewDefaultWriteOptions()
	store.ro = gorocksdb.NewDefaultReadOptions()
	store.ro.SetFillCache(false)

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
	wb := gorocksdb.NewWriteBatch()
	defer wb.Destroy()
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
	wb := gorocksdb.NewWriteBatch()
	defer wb.Destroy()
	wb.Delete([]byte(fmt.Sprintf("node/%d", n.GetId())))
	return s.db.Write(s.wo, wb)
}

func (s *Store) addNewWays(arr []*Way) error {
	wb := gorocksdb.NewWriteBatch()
	defer wb.Destroy()
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
	wb := gorocksdb.NewWriteBatch()
	defer wb.Destroy()
	wb.Delete([]byte(fmt.Sprintf("way/%d", n.GetId())))
	return s.db.Write(s.wo, wb)
}

func (s *Store) addNewRelations(arr []*Relation) error {
	wb := gorocksdb.NewWriteBatch()
	defer wb.Destroy()
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
	wb := gorocksdb.NewWriteBatch()
	defer wb.Destroy()
	s.indexer.removeRelation(n, wb)
	wb.Delete([]byte(fmt.Sprintf("relation/%d", n.GetId())))
	return s.db.Write(s.wo, wb)
}

func (s *Store) GetNode(id int64) (*Node, error) {
	n, err := s.db.Get(s.ro, []byte(fmt.Sprintf("node/%d", id)))
	if err != nil {
		return nil, err
	}

	if n.Size() == 0 {
		return nil, nil
	}

	node := &Node{}
	err = proto.Unmarshal(n.Data(), node)
	if err != nil {
		return nil, err
	}

	return node, nil
}

func (s *Store) GetWay(id int64) (*Way, error) {
	n, err := s.db.Get(s.ro, []byte(fmt.Sprintf("way/%d", id)))
	if err != nil {
		return nil, err
	}

	if n.Size() == 0 {
		return nil, nil
	}

	way := &Way{}
	err = proto.Unmarshal(n.Data(), way)
	if err != nil {
		return nil, err
	}

	return way, nil
}

func (s *Store) GetRelation(id int64) (*Relation, error) {
	n, err := s.db.Get(s.ro, []byte(fmt.Sprintf("relation/%d", id)))
	if err != nil {
		return nil, err
	}

	if n.Size() == 0 {
		return nil, nil
	}

	rel := &Relation{}
	err = proto.Unmarshal(n.Data(), rel)
	if err != nil {
		return nil, err
	}

	return rel, nil
}

func (s *Store) Extract(configPath, outPath string) error {
	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		return err
	}

	config := &ExtractConfig{}
	err = yaml.Unmarshal(data, config)
	if err != nil {
		return err
	}

	extractor := &Extractor{
		store:   s,
		config:  config,
		outPath: outPath,
	}

	return extractor.Run()
}
