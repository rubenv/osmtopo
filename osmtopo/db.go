package osmtopo

import (
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/rubenv/osmtopo/osmtopo/model"
	"github.com/tecbot/gorocksdb"
)

func (e *Env) openStore() error {
	// Determine max number of open files
	var rLimit syscall.Rlimit
	err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit)
	if err != nil {
		return err
	}
	maxOpen := int(rLimit.Cur - 100)

	storeFolder := path.Join(e.storePath, "ldb")
	err = os.MkdirAll(storeFolder, 0755)
	if err != nil {
		return err
	}

	opts := gorocksdb.NewDefaultOptions()
	bb := gorocksdb.NewDefaultBlockBasedTableOptions()
	bb.SetBlockCache(gorocksdb.NewLRUCache(3 << 30))
	bb.SetFilterPolicy(gorocksdb.NewBloomFilter(10))
	opts.SetCreateIfMissing(true)
	opts.SetBlockBasedTableFactory(bb)
	opts.SetMaxOpenFiles(maxOpen)
	opts.SetMaxBackgroundCompactions(1)
	db, err := gorocksdb.OpenDb(opts, storeFolder)
	if err != nil {
		return err
	}
	e.db = db

	e.wo = gorocksdb.NewDefaultWriteOptions()
	e.ro = gorocksdb.NewDefaultReadOptions()
	e.ro.SetFillCache(false)
	return nil
}

func (e *Env) getTimestamp(stamp string) (time.Time, error) {
	key := stampKey(stamp)
	n, err := e.db.Get(e.ro, key)
	if err != nil {
		return time.Time{}, err
	}
	defer n.Free()

	if n.Size() == 0 {
		return time.Time{}, nil
	}

	ts, err := time.Parse(time.RFC3339, string(n.Data()))
	if err != nil {
		return time.Time{}, err
	}
	return ts, nil
}

func (e *Env) setTimestamp(stamp string, ts time.Time) error {
	key := stampKey(stamp)
	t := ts.Format(time.RFC3339)
	wb := gorocksdb.NewWriteBatch()
	defer wb.Destroy()
	wb.Put(key, []byte(t))
	return e.db.Write(e.wo, wb)
}

func (e *Env) shouldRun(stamp string, every int64) (bool, error) {
	ts, err := e.getTimestamp(stamp)
	if err != nil {
		return false, err
	}

	nextRun := ts.Add(time.Duration(every) * time.Second)
	return !nextRun.After(time.Now()), nil
}

func (e *Env) getFlag(flag string) (bool, error) {
	key := flagKey(flag)
	n, err := e.db.Get(e.ro, key)
	if err != nil {
		return false, err
	}
	defer n.Free()

	if n.Size() == 0 {
		return false, nil
	}

	return string(n.Data()) == "1", nil
}

func (e *Env) setFlag(flag string, v bool) error {
	key := flagKey(flag)
	wb := gorocksdb.NewWriteBatch()
	defer wb.Destroy()
	if v {
		wb.Put(key, []byte("1"))
	} else {
		wb.Put(key, []byte("0"))
	}
	return e.db.Write(e.wo, wb)
}

func (e *Env) getInt(nbr string) (int64, error) {
	key := intKey(nbr)
	n, err := e.db.Get(e.ro, key)
	if err != nil {
		return 0, err
	}
	defer n.Free()

	if n.Size() == 0 {
		return 0, nil
	}

	v, err := strconv.ParseInt(string(n.Data()), 10, 64)
	if err != nil {
		return 0, err
	}
	return v, nil
}

func (e *Env) setInt(nbr string, v int64) error {
	key := intKey(nbr)
	wb := gorocksdb.NewWriteBatch()
	defer wb.Destroy()
	wb.Put(key, []byte(fmt.Sprintf("%d", v)))
	return e.db.Write(e.wo, wb)
}

func (e *Env) removeGeometry(prefix string, id int64) error {
	wb := gorocksdb.NewWriteBatch()
	defer wb.Destroy()
	key := fmt.Sprintf("geometry/%s/%d", prefix, id)
	wb.Delete([]byte(key))
	return e.db.Write(e.wo, wb)
}

func (e *Env) removeGeometries(prefix string) error {
	keys, err := e.GetGeometries(prefix)
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

	return e.db.Write(e.wo, wb)
}

func (e *Env) GetGeometries(prefix string) ([]int64, error) {
	ro := gorocksdb.NewDefaultReadOptions()
	ro.SetFillCache(false)

	it := e.db.NewIterator(ro)
	defer it.Close()

	result := make([]int64, 0)
	keyPrefix := fmt.Sprintf("geometry/%s/", prefix)
	it.Seek([]byte(keyPrefix))
	for it.Valid() {
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
		it.Next()
	}

	return result, nil
}

func (e *Env) GetGeometry(prefix string, id int64) (*model.Geometry, error) {
	n, err := e.db.Get(e.ro, []byte(fmt.Sprintf("geometry/%s/%d", prefix, id)))
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

func (e *Env) addGeometry(prefix string, n *model.Geometry) error {
	wb := gorocksdb.NewWriteBatch()
	defer wb.Destroy()
	data, err := n.Marshal()
	if err != nil {
		return err
	}
	key := fmt.Sprintf("geometry/%s/%d", prefix, n.Id)
	wb.Put([]byte(key), data)
	return e.db.Write(e.wo, wb)
}

func (e *Env) addNewGeometries(prefix string, arr []*model.Geometry) error {
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
	return e.db.Write(e.wo, wb)
}

func (e *Env) addNewNodes(arr []model.Node) error {
	wb := gorocksdb.NewWriteBatch()
	defer wb.Destroy()
	for _, n := range arr {
		data, err := n.Marshal()
		if err != nil {
			return err
		}
		wb.Put(nodeKey(n.Id), data)
	}
	return e.db.Write(e.wo, wb)
}

func (e *Env) addNewWays(arr []model.Way) error {
	wb := gorocksdb.NewWriteBatch()
	defer wb.Destroy()
	for _, n := range arr {
		data, err := n.Marshal()
		if err != nil {
			return err
		}
		wb.Put(wayKey(n.Id), data)
	}
	return e.db.Write(e.wo, wb)
}

func (e *Env) addNewRelations(arr []model.Relation) error {
	wb := gorocksdb.NewWriteBatch()
	defer wb.Destroy()
	for _, n := range arr {
		data, err := n.Marshal()
		if err != nil {
			return err
		}
		wb.Put(relationKey(n.Id), data)
	}
	return e.db.Write(e.wo, wb)
}

func (e *Env) removeNode(n model.Node) error {
	wb := gorocksdb.NewWriteBatch()
	defer wb.Destroy()
	wb.Delete(nodeKey(n.Id))
	return e.db.Write(e.wo, wb)
}

func (e *Env) removeWay(n model.Way) error {
	wb := gorocksdb.NewWriteBatch()
	defer wb.Destroy()
	wb.Delete(wayKey(n.Id))
	return e.db.Write(e.wo, wb)
}

func (e *Env) removeRelation(n model.Relation) error {
	wb := gorocksdb.NewWriteBatch()
	defer wb.Destroy()
	wb.Delete(relationKey(n.Id))
	return e.db.Write(e.wo, wb)
}

func (e *Env) GetNode(id int64) (*model.Node, error) {
	n, err := e.db.Get(e.ro, nodeKey(id))
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

func (e *Env) GetWay(id int64) (*model.Way, error) {
	n, err := e.db.Get(e.ro, wayKey(id))
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

func (e *Env) GetRelation(id int64) (*model.Relation, error) {
	n, err := e.db.Get(e.ro, relationKey(id))
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

func (e *Env) addMissing(arr []*model.MissingCoordinate) error {
	for _, c := range arr {
		c.EnsureID()
	}

	wb := gorocksdb.NewWriteBatch()
	defer wb.Destroy()
	for _, n := range arr {
		data, err := n.Marshal()
		if err != nil {
			return err
		}
		wb.Put(missingKey(n.Id), data)
	}
	return e.db.Write(e.wo, wb)
}

func (e *Env) countMissing() (int, error) {
	ro := gorocksdb.NewDefaultReadOptions()
	ro.SetFillCache(false)

	it := e.db.NewIterator(ro)
	defer it.Close()

	missing := 0
	keyPrefix := "missing/"
	it.Seek([]byte(keyPrefix))
	for it.Valid() {
		key := it.Key()
		k := key.Data()
		if !strings.HasPrefix(string(k), keyPrefix) {
			key.Free()
			break
		}

		key.Free()
		missing++
		it.Next()
	}

	return missing, nil
}

func (e *Env) getMissing() (*model.MissingCoordinate, error) {
	ro := gorocksdb.NewDefaultReadOptions()
	ro.SetFillCache(false)

	it := e.db.NewIterator(ro)
	defer it.Close()

	keyPrefix := []byte("missing/")
	it.Seek(keyPrefix)
	if !it.ValidForPrefix(keyPrefix) {
		return nil, nil
	}

	data := it.Value()
	defer data.Free()

	missing := &model.MissingCoordinate{}
	err := missing.Unmarshal(data.Data())
	if err != nil {
		return nil, err
	}

	return missing, nil
}

func (e *Env) removeMissing(n *model.MissingCoordinate) error {
	wb := gorocksdb.NewWriteBatch()
	defer wb.Destroy()
	wb.Delete(missingKey(n.Id))
	return e.db.Write(e.wo, wb)
}

type relationIter struct {
	it     *gorocksdb.Iterator
	prefix []byte
}

func (i *relationIter) Next() (*model.Relation, error) {
	if !i.it.ValidForPrefix(i.prefix) {
		return nil, nil
	}

	rel := &model.Relation{}
	data := i.it.Value()
	defer data.Free()

	err := rel.Unmarshal(data.Data())
	if err != nil {
		return nil, err

	}

	i.it.Next()
	return rel, nil
}

func (i *relationIter) Close() {
	i.it.Close()
}

func (e *Env) iterRelations() (*relationIter, error) {
	ro := gorocksdb.NewDefaultReadOptions()
	ro.SetFillCache(false)

	it := e.db.NewIterator(ro)
	it.Seek([]byte("relation/"))

	return &relationIter{
		it:     it,
		prefix: []byte("relation/"),
	}, nil
}
