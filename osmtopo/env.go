package osmtopo

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
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

	lookup *lookupData

	Status Status
}

type Status struct {
	Running     bool    `json:"running"`
	Initialized bool    `json:"initialized"`
	Missing     int     `json:"missing"`
	Config      *Config `json:"config"`
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

	env.Status.Config = config

	if !config.NoUpdate {
		env.done.Add(1)
		go env.runUpdater()
	}

	c, err := env.countMissing()
	if err != nil {
		return nil, err
	}
	env.Status.Missing = c

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
	mux.Handle("/api/missing", http.HandlerFunc(e.handleMissing))
	mux.Handle("/api/coordinate", http.HandlerFunc(e.handleCoordinate))
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
		e.Status.Running = true
		nextRun := time.Now().Add(1 * time.Hour)

		err := e.updateData()
		if err != nil {
			e.log("updater", "Failed: %s", err)
		} else {
			e.Status.Initialized = true
		}

		e.Status.Running = false

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
	// Water
	err := e.updateWater()
	if err != nil {
		return err
	}

	// OSM sources
	for name, source := range e.config.Sources {
		err := e.updateSource(name, source)
		if err != nil {
			return err
		}
	}

	// Build lookup trees
	lookup := newLookupData()
	levelNeeded := make(map[int]bool)
	levelTagNeeded := make(map[string]bool)
	for _, layer := range e.config.Layers {
		for _, admin := range layer.AdminLevels {
			if !levelNeeded[admin] {
				levelNeeded[admin] = true
				levelTagNeeded[fmt.Sprintf("%d", admin)] = true
			}
		}
	}

	it, err := e.iterRelations()
	if err != nil {
		return err
	}
	defer it.Close()

	for {
		rel, err := it.Next()
		if err != nil {
			return err
		}
		if rel == nil {
			break
		}

		a, ok := rel.GetTag("admin_level")
		if !ok || !levelTagNeeded[a] {
			continue
		}

		level, err := strconv.ParseInt(a, 10, 64)
		if err != nil {
			return err
		}

		g, err := ToGeometry(rel, e)
		if err != nil {
			// Broken geometry, skip!
			continue
		}

		geom, err := GeometryFromGeos(g)
		if err != nil {
			return err
		}

		err = lookup.IndexGeometry(int(level), rel.Id, geom)
		if err != nil {
			return err
		}
	}
	e.lookup = lookup

	return nil
}

func (e *Env) handleStatus(w http.ResponseWriter, req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(e.Status)
}

func (e *Env) handleMissing(w http.ResponseWriter, req *http.Request) {
	if req.Method != "POST" {
		http.Error(w, "Should send a POST request", http.StatusBadRequest)
		return
	}

	err := e.importMissing(req.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
}

func (e *Env) handleCoordinate(w http.ResponseWriter, req *http.Request) {
	c, err := e.getMissingCoordinate()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = json.NewEncoder(w).Encode(c)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
