package osmtopo

import (
	"fmt"
	"math"
	"runtime"
	"sync"

	geojson "github.com/paulmach/go.geojson"
	"github.com/rubenv/osmtopo/osmtopo/model"
	"github.com/rubenv/topojson"
	"golang.org/x/sync/errgroup"
)

type RelationFilterFunc func(rel *model.Relation) bool

type GeometryPipeline struct {
	id        int64
	env       *Env
	simplify  int
	quantize  float64
	clipwater bool
	accept    RelationFilterFunc
}

func NewGeometryPipeline(e *Env) *GeometryPipeline {
	return &GeometryPipeline{
		env: e,
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
				geometries <- out
			}
		})
	}
	g.Go(func() error {
		geomWg.Wait()
		close(geometries)
		return nil
	})

	// Do pre-simplification without quantization when clipping
	// water
	simplified := make(chan *geojson.FeatureCollection)
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
			topo := topojson.NewTopology(fc, &topojson.TopologyOptions{
				Simplify:   maxErr,
				IDProperty: "id",
			})
			simplified <- topo.ToGeoJSON()
		} else {
			simplified <- fc
		}
		return nil
	})

	// TODO: Clip water

	// Simplify again, this time using quantization
	quantized := make(chan *topojson.Topology, 1)
	g.Go(func() error {
		defer close(quantized)
		topo := topojson.NewTopology(<-simplified, &topojson.TopologyOptions{
			PostQuantize: p.quantize,
			Simplify:     maxErr,
			IDProperty:   "id",
		})
		quantized <- topo
		return nil
	})

	err := g.Wait()
	if err != nil {
		return nil, err
	}

	return <-quantized, nil
}