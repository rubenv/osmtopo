package osmtopo

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gobuffalo/packr"
	lru "github.com/hashicorp/golang-lru"
	"github.com/rubenv/osmtopo/osmtopo/lookup"
	"github.com/rubenv/osmtopo/osmtopo/model"
	"github.com/rubenv/servertiming"
	"github.com/rubenv/topojson"
	"github.com/tecbot/gorocksdb"
)

type Env struct {
	ctx  context.Context
	cf   context.CancelFunc
	done sync.WaitGroup

	initialized sync.WaitGroup

	config         *Config
	topologiesFile string
	storePath      string
	outputPath     string

	db *gorocksdb.DB
	wo *gorocksdb.WriteOptions
	ro *gorocksdb.ReadOptions

	lookup     *lookup.Data
	topologies *lookup.Data

	topoData  *TopologyData
	topoCache *lru.Cache

	waterLock     sync.Mutex
	waterClipGeos map[string][]*clipGeometry

	Status Status
}

type Status struct {
	Running     bool    `json:"running"`
	Initialized bool    `json:"initialized"`
	Missing     int     `json:"missing"`
	Config      *Config `json:"config"`

	Export ExportStatus `json:"export"`
}

type ExportStatus struct {
	Running bool   `json:"running"`
	Error   string `json:"error"`
}

func NewEnv(config *Config, topologiesFile, storePath, outputPath string) (*Env, error) {
	ctx, cf := context.WithCancel(context.Background())

	cache, err := lru.New(1024)
	if err != nil {
		return nil, err
	}

	env := &Env{
		ctx:            ctx,
		cf:             cf,
		config:         config,
		topologiesFile: topologiesFile,
		storePath:      storePath,
		outputPath:     outputPath,
		topoCache:      cache,
		waterClipGeos:  make(map[string][]*clipGeometry),
	}
	err = env.openStore()
	if err != nil {
		return nil, err
	}

	env.Status.Config = config

	env.done.Add(1)
	env.initialized.Add(1)
	go env.runUpdater()

	err = env.loadTopologies()
	if err != nil {
		return nil, err
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
	mux.Handle("/api/add", http.HandlerFunc(e.handleAdd))
	mux.Handle("/api/delete", http.HandlerFunc(e.handleDelete))
	mux.Handle("/api/export", http.HandlerFunc(e.handleExport))
	mux.Handle("/api/topologies", http.HandlerFunc(e.handleExportTopologies))
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
			if !e.Status.Initialized {
				e.initialized.Done()
				e.Status.Initialized = true
			}
		}

		e.Status.Running = false

		select {
		case <-time.After(time.Until(nextRun)):
		case <-done:
			return
		}
	}
}

func (e *Env) runExporter() {
	e.done.Add(1)
	defer e.done.Done()

	e.Status.Export.Running = true
	err := e.export()
	if err != nil {
		e.Status.Export.Error = err.Error()
	} else {
		e.Status.Export.Error = ""
	}
	e.Status.Export.Running = false
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

	// Refresh lookup
	err = e.loadLookup()
	if err != nil {
		return err
	}

	return nil
}

func (e *Env) loadLookup() error {
	lookup := lookup.New()
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
			return fmt.Errorf("GeometryPipeline: %s", err)
		}

		err = lookup.IndexTopology(layer.ID, topo)
		if err != nil {
			return err
		}
	}

	err := lookup.Build()
	if err != nil {
		return err
	}

	e.lookup = lookup

	return nil
}

func (e *Env) loadTopologies() error {
	topoData, err := ReadTopologies(e.topologiesFile)
	if err != nil {
		return err
	}
	e.topoData = topoData

	lookup := lookup.New()
	for _, layer := range e.config.Layers {
		ids, ok := e.topoData.Layers[layer.ID]
		if !ok {
			continue
		}

		idNeeded := make(map[int64]bool)
		for _, id := range ids {
			idNeeded[id] = true
		}

		pipe := NewGeometryPipeline(e).
			Filter(func(rel *model.Relation) bool {
				return idNeeded[rel.Id]
			}).
			Simplify(layer.Simplify)

		topo, err := pipe.Run()
		if err != nil {
			return fmt.Errorf("GeometryPipeline: %s", err)
		}

		err = lookup.IndexTopology(layer.ID, topo)
		if err != nil {
			return err
		}
	}

	err = lookup.Build()
	if err != nil {
		return err
	}

	e.topologies = lookup

	return nil
}

func (e *Env) getTopology(layerID string, id int64) (*topojson.Topology, *servertiming.Timing, error) {
	key := fmt.Sprintf("%s-%d", layerID, id)
	t, ok := e.topoCache.Get(key)
	if ok {
		return t.(*topojson.Topology), nil, nil
	}

	var layer Layer
	found := false
	for _, l := range e.config.Layers {
		if l.ID == layerID {
			found = true
			layer = l
		}
	}
	if !found {
		return nil, nil, fmt.Errorf("Unknown layer: %s", layerID)
	}

	pipe := NewGeometryPipeline(e).
		Select(id).
		Simplify(layer.Simplify).
		ClipWater().
		Quantize(1e6)

	topo, err := pipe.Run()
	if err != nil {
		return nil, nil, err
	}

	e.topoCache.Add(key, topo)
	return topo, pipe.Timing, nil
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

	id, err := strconv.ParseInt(parts[4], 10, 64)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	topo, timing, err := e.getTopology(parts[3], id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if timing != nil {
		w.Header().Set("Server-Timing", timing.String())
	}

	err = json.NewEncoder(w).Encode(topo)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (e *Env) handleAdd(w http.ResponseWriter, req *http.Request) {
	if req.Method != "POST" {
		http.Error(w, "Should send a POST request", http.StatusBadRequest)
		return
	}

	in := make(map[string]int64)

	err := json.NewDecoder(req.Body).Decode(&in)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	added := false
	for _, layer := range e.config.Layers {
		id, ok := in[layer.ID]
		if !ok {
			continue
		}

		e.topoData.Add(layer.ID, id)

		added = true
	}

	if added {
		err = e.topoData.WriteTo(e.topologiesFile)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		err = e.loadTopologies()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

func (e *Env) handleDelete(w http.ResponseWriter, req *http.Request) {
	if req.Method != "POST" {
		http.Error(w, "Should send a POST request", http.StatusBadRequest)
		return
	}

	in := &model.MissingCoordinate{}

	err := json.NewDecoder(req.Body).Decode(&in)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = e.removeMissing(in)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	e.Status.Missing--
}

func (e *Env) handleExport(w http.ResponseWriter, req *http.Request) {
	if req.Method != "POST" {
		http.Error(w, "Should send a POST request", http.StatusBadRequest)
		return
	}

	go e.runExporter()
}

func (e *Env) handleExportTopologies(w http.ResponseWriter, req *http.Request) {
	if e.Status.Export.Running {
		http.Error(w, "Export is currently running", http.StatusBadRequest)
		return
	}
	if e.Status.Export.Error != "" {
		http.Error(w, fmt.Sprintf("Export failed: %s", e.Status.Export.Error), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/gzip")
	gz := gzip.NewWriter(w)
	defer gz.Close()

	tw := tar.NewWriter(gz)
	defer tw.Close()

	writeErrorFile := func(err error) {
		str := fmt.Sprintf("Archive failed: %s", err)
		tw.WriteHeader(&tar.Header{
			Name: "error",
			Mode: 0600,
			Size: int64(len(str)),
		})
		tw.Write([]byte(str))
	}

	copyFile := func(filename string) error {
		fp, err := os.Open(filename)
		if err != nil {
			return err
		}
		defer fp.Close()

		_, err = io.Copy(tw, fp)
		return err
	}

	levels, err := ioutil.ReadDir(e.outputPath)
	if err != nil {
		writeErrorFile(err)
		return
	}

	for _, level := range levels {
		if !level.IsDir() || strings.HasPrefix(level.Name(), ".") {
			continue
		}

		folder := path.Join(e.outputPath, level.Name())

		files, err := ioutil.ReadDir(folder)
		if err != nil {
			writeErrorFile(err)
			return
		}

		for _, file := range files {
			filename := path.Join(folder, file.Name())
			if !strings.HasSuffix(filename, ".topojson") {
				continue
			}

			err = tw.WriteHeader(&tar.Header{
				Name: fmt.Sprintf("%s/%s", level.Name(), file.Name()),
				Mode: 0600,
				Size: file.Size(),
			})
			if err != nil {
				writeErrorFile(err)
				return
			}

			err = copyFile(filename)
			if err != nil {
				writeErrorFile(err)
				return
			}
		}
	}
}
