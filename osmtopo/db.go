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
	key := fmt.Sprintf("stamp/%s", stamp)
	n, err := e.db.Get(e.ro, []byte(key))
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
	key := fmt.Sprintf("stamp/%s", stamp)
	t := ts.Format(time.RFC3339)
	wb := gorocksdb.NewWriteBatch()
	defer wb.Destroy()
	wb.Put([]byte(key), []byte(t))
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
