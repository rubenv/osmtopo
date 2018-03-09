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
