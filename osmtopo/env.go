package osmtopo

import (
	"context"
	"net/http"
	"os"
	"path"
	"sync"
	"syscall"
	"time"

	"github.com/tecbot/gorocksdb"
)

type Env struct {
	ctx  context.Context
	cf   context.CancelFunc
	done sync.WaitGroup

	config         *Config
	topologiesFile string
	storePath      string

	db *gorocksdb.DB
	wo *gorocksdb.WriteOptions
	ro *gorocksdb.ReadOptions
}

func NewEnv(config *Config, topologiesFile, storePath string) (*Env, error) {
	ctx, cf := context.WithCancel(context.Background())

	env := &Env{
		ctx:            ctx,
		cf:             cf,
		config:         config,
		topologiesFile: topologiesFile,
		storePath:      storePath,
	}
	err := env.openStore()
	if err != nil {
		return nil, err
	}
	return env, nil
}

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

func (e *Env) Stop() {
	e.cf()
	e.done.Wait()
	e.db.Close()
}

func (e *Env) StartServer(listen string) error {
	e.done.Add(1)
	defer e.done.Done()

	s := &http.Server{
		Addr:           listen,
		Handler:        nil,
		ReadTimeout:    60 * time.Second,
		WriteTimeout:   60 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	go func() {
		<-e.ctx.Done()
		ctx, _ := context.WithTimeout(context.Background(), 15*time.Second)
		s.Shutdown(ctx)
	}()

	err := s.ListenAndServe()
	if err == http.ErrServerClosed {
		err = nil
	}
	return err
}
