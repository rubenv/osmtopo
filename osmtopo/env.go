package osmtopo

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gobuffalo/packr"
	"github.com/kr/pretty"
	"github.com/rubenv/osmtopo/osmtopo/model"
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
	mux.Handle("/api/topo/", http.HandlerFunc(e.handleTopo))
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

	lookup := newLookupData(e)
	for _, layer := range e.config.Layers {
		levelNeeded := make(map[int]bool)
		for _, admin := range layer.AdminLevels {
			levelNeeded[admin] = true
		}

		pipe := NewGeometryPipeline(e).
			Filter(func(rel *model.Relation) bool {
				return levelNeeded[rel.GetAdminLevel()]
			}).
			Simplify(layer.Simplify)

		topo, err := pipe.Run()
		if err != nil {
			return err
		}

		fc := topo.ToGeoJSON()
		for _, feat := range fc.Features {
			id, err := strconv.ParseInt(feat.ID.(string), 10, 64)
			if err != nil {
				return err
			}

			err = lookup.IndexGeometry(layer.ID, id, feat.Geometry)
			if err != nil {
				return err
			}
		}
	}
	e.lookup = lookup
	pretty.Log("Done")

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

func (e *Env) handleTopo(w http.ResponseWriter, req *http.Request) {
	parts := strings.Split(req.URL.Path, "/")
	if len(parts) != 5 {
		http.Error(w, "Missing ID", http.StatusNotFound)
		return
	}

	var layer Layer
	found := false
	for _, l := range e.config.Layers {
		if l.ID == parts[3] {
			found = true
			layer = l
		}
	}
	if !found {
		http.Error(w, "Unknown layer", http.StatusNotFound)
		return
	}

	id, err := strconv.ParseInt(parts[4], 10, 64)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	pipe := NewGeometryPipeline(e).
		Select(id).
		Simplify(layer.Simplify).
		Quantize(1e6)

	topo, err := pipe.Run()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = json.NewEncoder(w).Encode(topo)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
