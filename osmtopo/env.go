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
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gobuffalo/packr"
	lru "github.com/hashicorp/golang-lru"
	"github.com/paulsmith/gogeos/geos"
	"github.com/rubenv/osmtopo/osmtopo/lookup"
	"github.com/rubenv/osmtopo/osmtopo/model"
	"github.com/rubenv/servertiming"
	"github.com/rubenv/topojson"
	"github.com/tecbot/gorocksdb"
	"golang.org/x/sync/errgroup"
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
	geosCache *lru.Cache

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
	env, err := prepareEnv(config, topologiesFile, storePath, outputPath)
	if err != nil {
		return nil, err
	}

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

// Used for testing
func prepareEnv(config *Config, topologiesFile, storePath, outputPath string) (*Env, error) {
	ctx, cf := context.WithCancel(context.Background())

	topoCache, err := lru.New(1024)
	if err != nil {
		return nil, err
	}

	geosCache, err := lru.New(1024)
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
		topoCache:      topoCache,
		geosCache:      geosCache,
		waterClipGeos:  make(map[string][]*clipGeometry),
	}
	err = env.openStore()
	if err != nil {
		return nil, err
	}

	env.Status.Config = config

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
	mux.Handle("/api/coverage/", http.HandlerFunc(e.handleCoverage))
	mux.Handle("/api/geometry/", http.HandlerFunc(e.handleGeometry))
	mux.Handle("/api/relation/", http.HandlerFunc(e.handleRelation))
	mux.Handle("/api/way/", http.HandlerFunc(e.handleWay))
	mux.Handle("/api/node/", http.HandlerFunc(e.handleNode))
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
	lookupData := lookup.New()
	for _, layer := range e.config.Layers {
		levelNeeded := make(map[int]bool)
		for _, admin := range layer.AdminLevels {
			levelNeeded[admin] = true
		}

		relations := make(chan *model.Relation, 100)

		g, ctx := errgroup.WithContext(e.ctx)
		g.Go(func() error {
			defer close(relations)

			it, err := e.iterRelations()
			if err != nil {
				return err
			}
			defer it.Close()

			for ctx.Err() == nil {
				rel, err := it.Next()
				if err != nil {
					return err
				}
				if rel == nil {
					break
				}

				relations <- rel
			}

			return ctx.Err()
		})

		geomWorkers := runtime.NumCPU() * 2
		geomWg := sync.WaitGroup{}
		geomWg.Add(geomWorkers)
		for i := 0; i < geomWorkers; i++ {
			g.Go(func() error {
				defer geomWg.Done()
				for ctx.Err() == nil {
					rel, ok := <-relations
					if !ok {
						return nil
					}

					if !levelNeeded[rel.GetAdminLevel()] {
						continue
					}

					cov, err := e.GetS2Coverage(rel.Id)
					if err != nil {
						return err
					}

					if cov == nil || len(cov) == 0 {
						geom, err := ToGeometryCached("rel", rel, e)
						if err != nil {
							// Broken geometry, skip!
							continue
						}

						c, err := lookup.GeometryToCoverage(geom)
						if err != nil {
							return err
						}
						cov = c

						err = e.addS2Coverage(rel.Id, cov)
						if err != nil {
							return err
						}
					}

					for _, cu := range cov {
						err = lookupData.IndexCells(layer.ID, rel.Id, cu)
						if err != nil {
							return err
						}
					}
				}

				return ctx.Err()
			})
		}

		err := g.Wait()
		if err != nil {
			return err
		}
	}

	err := lookupData.Build()
	if err != nil {
		return err
	}

	e.lookup = lookupData

	return nil
}

func (e *Env) loadTopologies() error {
	topoData, err := ReadTopologies(e.topologiesFile)
	if err != nil {
		return err
	}
	e.topoData = topoData

	var g errgroup.Group

	lookup := lookup.New()
	for _, l := range e.config.Layers {
		layer := l

		ids, ok := e.topoData.Layers[layer.ID]
		if !ok {
			continue
		}

		g.Go(func() error {
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

			return lookup.IndexFeatures(layer.ID, topo.ToGeoJSON())
		})
	}

	err = g.Wait()
	if err != nil {
		return err
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

func (e *Env) queryLookup(lookup *lookup.Data, lat, lon float64, layer string) ([]int64, error) {
	matches, err := lookup.Query(lat, lon, layer)
	if err != nil {
		return nil, err
	}

	point, err := geos.NewPoint(geos.Coord{X: lon, Y: lat})
	if err != nil {
		return nil, err
	}

	result := make([]int64, 0, len(matches))
	for _, match := range matches {
		key := fmt.Sprintf("%d", match)
		var g *geos.Geometry

		o, ok := e.geosCache.Get(key)
		if ok {
			g = o.(*geos.Geometry)
		} else {
			rel, err := e.GetRelation(match)
			if err != nil {
				return nil, err
			}

			geom, err := ToGeometryCached("rel", rel, e)
			if err != nil {
				return nil, fmt.Errorf("Fetch geometry: %s on rel %d", err, rel.Id)
			}

			o, err := GeometryToGeos(geom)
			if err != nil {
				return nil, fmt.Errorf("Convert to geos: %s on rel %d", err, rel.Id)
			}
			g = o
			e.geosCache.Add(key, g)
		}

		contains, err := g.Contains(point)
		if err != nil {
			geom, e := GeometryFromGeos(g)
			if e != nil {
				return nil, e
			}
			d, e := json.Marshal(geom)
			if e != nil {
				return nil, e
			}
			fmt.Println(string(d))
			return nil, fmt.Errorf("Contains: %s on rel %d", err, match)
		}

		if contains {
			result = append(result, match)
		}
	}

	return result, nil
}

func (e *Env) handleCoverage(w http.ResponseWriter, req *http.Request) {
	parts := strings.Split(req.URL.Path, "/")
	if len(parts) != 4 {
		http.Error(w, "Missing ID", http.StatusNotFound)
		return
	}

	id, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	switch req.Method {
	case "GET":
		cov, err := e.GetS2Coverage(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if cov == nil {
			http.Error(w, "Not Found", http.StatusNotFound)
		}

		err = json.NewEncoder(w).Encode(cov)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	case "DELETE":
		err = e.removeS2Coverage(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	default:
		http.Error(w, fmt.Sprintf("Method not allowed: %s", req.Method), http.StatusBadRequest)
		return
	}
}

func (e *Env) handleGeometry(w http.ResponseWriter, req *http.Request) {
	parts := strings.Split(req.URL.Path, "/")
	if len(parts) != 4 {
		http.Error(w, "Missing ID", http.StatusNotFound)
		return
	}

	id, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	switch req.Method {
	case "GET":
		geom, err := e.GetGeometry("rel", id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if geom == nil {
			http.Error(w, "Not Found", http.StatusNotFound)
		}

		_, err = w.Write(geom.Geojson)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	case "DELETE":
		err = e.removeGeometry("rel", id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	default:
		http.Error(w, fmt.Sprintf("Method not allowed: %s", req.Method), http.StatusBadRequest)
		return
	}
}

func (e *Env) handleRelation(w http.ResponseWriter, req *http.Request) {
	if req.Method != "GET" {
		http.Error(w, "Should send a GET request", http.StatusBadRequest)
		return
	}

	parts := strings.Split(req.URL.Path, "/")
	if len(parts) != 4 {
		http.Error(w, "Missing ID", http.StatusNotFound)
		return
	}

	id, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	rel, err := e.GetRelation(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if rel == nil {
		http.Error(w, "Not Found", http.StatusNotFound)
	}

	json.NewEncoder(w).Encode(rel)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (e *Env) handleWay(w http.ResponseWriter, req *http.Request) {
	if req.Method != "GET" {
		http.Error(w, "Should send a GET request", http.StatusBadRequest)
		return
	}

	parts := strings.Split(req.URL.Path, "/")
	if len(parts) != 4 {
		http.Error(w, "Missing ID", http.StatusNotFound)
		return
	}

	id, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	way, err := e.GetWay(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if way == nil {
		http.Error(w, "Not Found", http.StatusNotFound)
	}

	json.NewEncoder(w).Encode(way)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (e *Env) handleNode(w http.ResponseWriter, req *http.Request) {
	if req.Method != "GET" {
		http.Error(w, "Should send a GET request", http.StatusBadRequest)
		return
	}

	parts := strings.Split(req.URL.Path, "/")
	if len(parts) != 4 {
		http.Error(w, "Missing ID", http.StatusNotFound)
		return
	}

	id, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	node, err := e.GetNode(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if node == nil {
		http.Error(w, "Not Found", http.StatusNotFound)
	}

	json.NewEncoder(w).Encode(node)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
