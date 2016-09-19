package osmtopo

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/omniscale/imposm3/parser/pbf"
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
	stat, err := os.Stat(file)
	if err != nil {
		return err
	}

	key := fmt.Sprintf("%s-%s", stat.Name(), stat.ModTime().Format(time.RFC3339))

	f, err := pbf.Open(file)
	if err != nil {
		return err
	}

	i := Import{
		Store:    s,
		File:     f,
		StateKey: key,
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

func (s *Store) addNewNodes(arr []*model.Node) error {
	wb := gorocksdb.NewWriteBatch()
	defer wb.Destroy()
	for _, n := range arr {
		data, err := n.Marshal()
		if err != nil {
			return err
		}
		wb.Put([]byte(fmt.Sprintf("node/%d", n.Id)), data)
	}
	return s.db.Write(s.wo, wb)
}

func (s *Store) removeNode(n *model.Node) error {
	wb := gorocksdb.NewWriteBatch()
	defer wb.Destroy()
	wb.Delete([]byte(fmt.Sprintf("node/%d", n.Id)))
	return s.db.Write(s.wo, wb)
}

func (s *Store) addNewWays(arr []*model.Way) error {
	wb := gorocksdb.NewWriteBatch()
	defer wb.Destroy()
	for _, n := range arr {
		data, err := n.Marshal()
		if err != nil {
			return err
		}
		wb.Put([]byte(fmt.Sprintf("way/%d", n.Id)), data)
	}
	return s.db.Write(s.wo, wb)
}

func (s *Store) removeWay(n *model.Way) error {
	wb := gorocksdb.NewWriteBatch()
	defer wb.Destroy()
	wb.Delete([]byte(fmt.Sprintf("way/%d", n.Id)))
	return s.db.Write(s.wo, wb)
}

func (s *Store) addNewRelations(arr []*model.Relation) error {
	wb := gorocksdb.NewWriteBatch()
	defer wb.Destroy()
	for _, n := range arr {
		data, err := n.Marshal()
		if err != nil {
			return err
		}
		wb.Put([]byte(fmt.Sprintf("relation/%d", n.Id)), data)
	}
	return s.db.Write(s.wo, wb)
}

func (s *Store) removeRelation(n *model.Relation) error {
	wb := gorocksdb.NewWriteBatch()
	defer wb.Destroy()
	wb.Delete([]byte(fmt.Sprintf("relation/%d", n.Id)))
	return s.db.Write(s.wo, wb)
}

func (s *Store) addNewGeometries(prefix string, arr []*model.Geometry) error {
	wb := gorocksdb.NewWriteBatch()
	defer wb.Destroy()
	for _, n := range arr {
		data, err := n.Marshal()
		if err != nil {
			return err
		}
		key := fmt.Sprintf("geometry/%s/%d", prefix, n.Id)
		wb.Put([]byte(key), data)
	}
	return s.db.Write(s.wo, wb)
}

func (s *Store) removeGeometries(prefix string) error {
	keys, err := s.GetGeometries(prefix)
	if err != nil {
		return err
	}
	if len(keys) == 0 {
		return nil
	}

	wb := gorocksdb.NewWriteBatch()
	defer wb.Destroy()

	for _, k := range keys {
		key := fmt.Sprintf("geometry/%s/%d", prefix, k)
		wb.Delete([]byte(key))
	}

	return s.db.Write(s.wo, wb)
}

func (s *Store) GetNode(id int64) (*model.Node, error) {
	n, err := s.db.Get(s.ro, []byte(fmt.Sprintf("node/%d", id)))
	if err != nil {
		return nil, err
	}
	defer n.Free()

	if n.Size() == 0 {
		return nil, nil
	}

	node := &model.Node{}
	err = node.Unmarshal(n.Data())
	if err != nil {
		return nil, err
	}

	return node, nil
}

func (s *Store) GetWay(id int64) (*model.Way, error) {
	n, err := s.db.Get(s.ro, []byte(fmt.Sprintf("way/%d", id)))
	if err != nil {
		return nil, err
	}
	defer n.Free()

	if n.Size() == 0 {
		return nil, nil
	}

	way := &model.Way{}
	err = way.Unmarshal(n.Data())
	if err != nil {
		return nil, err
	}

	return way, nil
}

func (s *Store) GetRelation(id int64) (*model.Relation, error) {
	n, err := s.db.Get(s.ro, []byte(fmt.Sprintf("relation/%d", id)))
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

func (s *Store) GetGeometries(prefix string) ([]int64, error) {
	ro := gorocksdb.NewDefaultReadOptions()
	ro.SetFillCache(false)

	it := s.db.NewIterator(ro)
	defer it.Close()

	result := make([]int64, 0)
	keyPrefix := fmt.Sprintf("geometry/%s/", prefix)
	it.Seek([]byte(keyPrefix))
	for it = it; it.Valid(); it.Next() {
		key := it.Key()
		k := key.Data()
		if !strings.HasPrefix(string(k), keyPrefix) {
			key.Free()
			break
		}

		key.Free()

		id, err := strconv.ParseInt(string(k[len(keyPrefix):]), 10, 64)
		if err != nil {
			return nil, err
		}
		result = append(result, id)
	}

	return result, nil
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

func (s *Store) GetImportState(key string) (*model.ImportState, error) {
	n, err := s.db.Get(s.ro, []byte(fmt.Sprintf("imports/%s", key)))
	if err != nil {
		return nil, err
	}
	defer n.Free()

	state := &model.ImportState{}
	err = state.Unmarshal(n.Data())
	if err != nil {
		return nil, err
	}
	return state, nil
}

func (s *Store) SetImportState(key string, state *model.ImportState) error {
	data, err := state.Marshal()
	if err != nil {
		return err
	}

	wb := gorocksdb.NewWriteBatch()
	defer wb.Destroy()
	wb.Put([]byte(fmt.Sprintf("imports/%s", key)), data)
	return s.db.Write(s.wo, wb)
}
