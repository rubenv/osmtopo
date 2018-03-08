package osmtopo

import (
	"fmt"
	"os"
	"syscall"

	"github.com/rubenv/osmtopo/osmtopo/model"
	"github.com/tecbot/gorocksdb"
)

type Store struct {
	path string
	db   *gorocksdb.DB

	wo *gorocksdb.WriteOptions
	ro *gorocksdb.ReadOptions
}

func NewStore(path string) (*Store, error) {
	// Determine max number of open files
	var rLimit syscall.Rlimit
	err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit)
	if err != nil {
		return nil, err
	}
	maxOpen := int(rLimit.Cur - 100)

	err = os.MkdirAll(path+"/ldb", 0755)
	if err != nil {
		return nil, err
	}

	store := &Store{
		path: path,
	}

	opts := gorocksdb.NewDefaultOptions()
	bb := gorocksdb.NewDefaultBlockBasedTableOptions()
	bb.SetBlockCache(gorocksdb.NewLRUCache(3 << 30))
	bb.SetFilterPolicy(gorocksdb.NewBloomFilter(10))
	opts.SetCreateIfMissing(true)
	opts.SetBlockBasedTableFactory(bb)
	opts.SetMaxOpenFiles(maxOpen)
	opts.SetMaxBackgroundCompactions(1)
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
	i := Import{
		Store:    s,
		Filename: file,
	}
	return i.Run()
}

func (s *Store) addNewNodes(arr []model.Node) error {
	wb := gorocksdb.NewWriteBatch()
	defer wb.Destroy()
	for _, n := range arr {
		data, err := n.Marshal()
		if err != nil {
			return err
		}
		wb.Put(nodeKey(n.Id), data)
	}
	return s.db.Write(s.wo, wb)
}

func (s *Store) removeNode(n *model.Node) error {
	wb := gorocksdb.NewWriteBatch()
	defer wb.Destroy()
	wb.Delete(nodeKey(n.Id))
	return s.db.Write(s.wo, wb)
}

func (s *Store) addNewWays(arr []model.Way) error {
	wb := gorocksdb.NewWriteBatch()
	defer wb.Destroy()
	for _, n := range arr {
		data, err := n.Marshal()
		if err != nil {
			return err
		}
		wb.Put(wayKey(n.Id), data)
	}
	return s.db.Write(s.wo, wb)
}

func (s *Store) removeWay(n *model.Way) error {
	wb := gorocksdb.NewWriteBatch()
	defer wb.Destroy()
	wb.Delete(wayKey(n.Id))
	return s.db.Write(s.wo, wb)
}

func (s *Store) removeRelation(n *model.Relation) error {
	wb := gorocksdb.NewWriteBatch()
	defer wb.Destroy()
	wb.Delete(relationKey(n.Id))
	return s.db.Write(s.wo, wb)
}

func (s *Store) GetRelation(id int64) (*model.Relation, error) {
	n, err := s.db.Get(s.ro, relationKey(id))
	if err != nil {
		return nil, err
	}
	defer n.Free()

	if n.Size() == 0 {
		return nil, nil
	}

	rel := &model.Relation{}
	err = rel.Unmarshal(n.Data())
	if err != nil {
		return nil, err
	}

	return rel, nil
}

func (s *Store) GetGeometry(prefix string, id int64) (*model.Geometry, error) {
	n, err := s.db.Get(s.ro, []byte(fmt.Sprintf("geometry/%s/%d", prefix, id)))
	if err != nil {
		return nil, err
	}
	defer n.Free()

	if n.Size() == 0 {
		return nil, nil
	}

	rel := &model.Geometry{}
	err = rel.Unmarshal(n.Data())
	if err != nil {
		return nil, err
	}

	return rel, nil
}

func (s *Store) Extract(configPath, outPath string) error {
	config, err := ParseConfig(configPath)
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

func (s *Store) Water() *Water {
	return &Water{store: s}
}

func (s *Store) Replicate(planet_file string) error {
	return Replicate(s, planet_file)
}

func (s *Store) GetConfig(key string) (string, error) {
	n, err := s.db.Get(s.ro, []byte(fmt.Sprintf("config/%s", key)))
	if err != nil {
		return "", err
	}
	defer n.Free()

	return string(n.Data()), nil
}

func (s *Store) SetConfig(key, value string) error {
	wb := gorocksdb.NewWriteBatch()
	defer wb.Destroy()
	wb.Put([]byte(fmt.Sprintf("config/%s", key)), []byte(value))
	return s.db.Write(s.wo, wb)
}

/*
func (s *Store) Resolve(configPath string, lat, lon float64) ([]ResolvedCoordinate, error) {
	config, err := ParseConfig(configPath)
	if err != nil {
		return nil, err
	}
	pretty.Log(config)

	return nil, nil
}
*/
