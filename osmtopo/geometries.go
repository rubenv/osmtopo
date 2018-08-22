package osmtopo

import (
	"fmt"
	"math"
	"runtime"
	"sync"

	geojson "github.com/paulmach/go.geojson"
	"github.com/paulsmith/gogeos/geos"
	"github.com/rubenv/osmtopo/osmtopo/model"
	"github.com/rubenv/servertiming"
	"github.com/rubenv/topojson"
	"golang.org/x/sync/errgroup"
)

type RelationFilterFunc func(rel *model.Relation) bool

type clipGeometry struct {
	Geometry *geos.Geometry
	Prepared *geos.PGeometry
}

type GeometryPipeline struct {
	id        int64
	env       *Env
	simplify  int
	quantize  float64
	clipwater bool
	accept    RelationFilterFunc

	Timing *servertiming.Timing
}

func NewGeometryPipeline(e *Env) *GeometryPipeline {
	return &GeometryPipeline{
		env:    e,
		Timing: servertiming.New().EnablePrefix(),
	}
}

func (p *GeometryPipeline) Select(id int64) *GeometryPipeline {
	p.id = id
	return p
}

func (p *GeometryPipeline) Simplify(simplify int) *GeometryPipeline {
	p.simplify = simplify
	return p
}

func (p *GeometryPipeline) Quantize(quantize float64) *GeometryPipeline {
	p.quantize = quantize
	return p
}

func (p *GeometryPipeline) ClipWater() *GeometryPipeline {
	p.clipwater = true
	return p
}

func (p *GeometryPipeline) Filter(accept RelationFilterFunc) *GeometryPipeline {
	p.accept = accept
	return p
}

func (p *GeometryPipeline) Run() (*topojson.Topology, error) {
	var g errgroup.Group

	// Load and pre-simplify geometries
	relations := make(chan *model.Relation, 100)
	geometries := make(chan *geojson.Feature, 100)
	p.Timing.Start("load", "Load geometries")
	if p.id == 0 {
		g.Go(func() error {
			defer close(relations)

			it, err := p.env.iterRelations()
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

				relations <- rel
			}

			return nil
		})
	} else {
		g.Go(func() error {
			defer close(relations)

			rel, err := p.env.GetRelation(p.id)
			if err != nil {
				return err
			}
			if rel == nil {
				return fmt.Errorf("Unknown relation: %d", p.id)
			}
			relations <- rel
			return nil
		})
	}

	geomWorkers := runtime.NumCPU()
	geomWg := sync.WaitGroup{}
	geomWg.Add(geomWorkers)
	for i := 0; i < geomWorkers; i++ {
		g.Go(func() error {
			defer geomWg.Done()
			for {
				rel, ok := <-relations
				if !ok {
					return nil
				}

				if p.accept != nil && !p.accept(rel) {
					continue
				}

				g, err := ToGeometry(rel, p.env)
				if err != nil {
					// Broken geometry, skip!
					continue
				}

				geom, err := GeometryFromGeos(g)
				if err != nil {
					return fmt.Errorf("GeometryFromGeos: %s for relation %d: %#v", err, rel.Id, g)
				}

				out := geojson.NewFeature(geom)
				out.SetProperty("id", fmt.Sprintf("%d", rel.Id))
				out.BoundingBox = geom.BoundingBox
				geometries <- out
			}
		})
	}
	g.Go(func() error {
		geomWg.Wait()
		close(geometries)
		p.Timing.Stop("load")
		return nil
	})

	// Do pre-simplification without quantization when clipping
	// water
	simplified := make(chan *geojson.FeatureCollection, 1)
	maxErr := float64(0)
	if p.simplify > 0 {
		maxErr = math.Pow(10, float64(-p.simplify))
	}
	g.Go(func() error {
		defer close(simplified)

		fc := geojson.NewFeatureCollection()
		for {
			f, ok := <-geometries
			if !ok {
				break
			}
			fc.AddFeature(f)
		}

		if p.simplify > 0 && p.clipwater {
			p.Timing.Start("pre-simplify", "Pre-clipping simplification")
			topo := topojson.NewTopology(fc, &topojson.TopologyOptions{
				Simplify:   maxErr,
				IDProperty: "id",
			})
			simplified <- topo.ToGeoJSON()
			p.Timing.Stop("pre-simplify")
		} else {
			simplified <- fc
		}
		return nil
	})

	clipped := make(chan *geojson.FeatureCollection, 1)
	g.Go(func() error {
		defer close(clipped)
		in := <-simplified

		if !p.clipwater {
			clipped <- in
			return nil
		}

		p.Timing.Start("loadwater", "Load water")
		clipGeos, err := p.env.loadWaterClipGeos(maxErr)
		if err != nil {
			return err
		}
		p.Timing.Stop("loadwater")

		p.Timing.Start("clip", "Clip water")
		defer p.Timing.Stop("clip")

		out := geojson.NewFeatureCollection()
		for _, feat := range in.Features {
			geom, err := GeometryToGeos(feat.Geometry)
			if err != nil {
				return err
			}

			// Apply a buffer to avoid self-intersections
			geom, err = geom.Buffer(0)
			if err != nil {
				return err
			}

			for _, clipGeom := range clipGeos {
				intersects, err := clipGeom.Prepared.Intersects(geom)
				if err != nil {
					return err
				}

				if intersects {
					clipped, err := geom.Difference(clipGeom.Geometry)
					// We ignore clipping errors here, these may happen when a
					// self-intersection occurs
					if err != nil {
						return fmt.Errorf("Failed to clip %s: %s\n", feat.ID, err)
					}
					geom = clipped
				}
			}

			g, err := GeometryFromGeos(geom)
			if err != nil {
				return err
			}
			feat.Geometry = g
			out.AddFeature(feat)
		}
		clipped <- out

		return nil
	})

	// Simplify again, this time using quantization
	quantized := make(chan *topojson.Topology, 1)
	g.Go(func() error {
		defer close(quantized)
		fc := <-clipped
		p.Timing.Start("post-simplify", "Post-clipping simplification")
		topo := topojson.NewTopology(fc, &topojson.TopologyOptions{
			PostQuantize: p.quantize,
			Simplify:     maxErr,
			IDProperty:   "id",
		})
		p.Timing.Stop("post-simplify")
		quantized <- topo
		return nil
	})

	err := g.Wait()
	if err != nil {
		return nil, err
	}

	return <-quantized, nil
}
