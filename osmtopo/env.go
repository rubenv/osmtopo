package osmtopo

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gobuffalo/packr"
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

	updateStatus UpdateStatus
}

type UpdateStatus struct {
	Running     bool `json:"running"`
	Initialized bool `json:"initialized"`
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

	if !config.NoUpdate {
		env.done.Add(1)
		go env.runUpdater()
	}

	return env, nil
}

func (e *Env) Stop() {
	e.cf()
	e.done.Wait()
	e.db.Close()
}

func (e *Env) StartServer(listen string) error {
	e.done.Add(1)
	defer e.done.Done()

	mux := http.NewServeMux()
	mux.Handle("/api/status", http.HandlerFunc(e.handleStatus))
	mux.Handle("/", http.FileServer(packr.NewBox("../frontend/build")))

	s := &http.Server{
		Addr:           listen,
		Handler:        mux,
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

func (e *Env) runUpdater() {
	defer e.done.Done()

	done := e.ctx.Done()
	for {
		e.updateStatus.Running = true
		nextRun := time.Now().Add(1 * time.Hour)

		err := e.updateData()
		if err != nil {
			e.log("updater", "Failed: %s", err)
		} else {
			e.updateStatus.Initialized = true
		}

		e.updateStatus.Running = false

		select {
		case <-time.After(time.Until(nextRun)):
		case <-done:
			return
		}
	}
}

func (e *Env) log(section, str string, args ...interface{}) {
	log.Printf(fmt.Sprintf("[%s] %s", section, str), args...)
}

func (e *Env) updateData() error {
	err := e.updateWater()
	if err != nil {
		return err
	}

	for name, source := range e.config.Sources {
		err := e.updateSource(name, source)
		if err != nil {
			return err
		}
	}

	return nil
}

func (e *Env) handleStatus(w http.ResponseWriter, req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(e.updateStatus)
}
