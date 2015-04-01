package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/paulsmith/gogeos/geos"
	"github.com/rubenv/osmtopo"
	"github.com/rubenv/osmtopo/geojson"
	"github.com/rubenv/osmtopo/simplify"
)

func main() {
	if len(os.Args) != 3 {
		log.Println("Usage: osmtopo-get-feature /path/to/datastore id")
		os.Exit(1)
	}

	err := do()
	if err != nil {
		log.Printf(err.Error())
		os.Exit(1)
	}

	os.Exit(0)
}

func do() error {
	store, err := osmtopo.NewStore(os.Args[1])
	if err != nil {
		return fmt.Errorf("Failed to open store: %s\n", err.Error())
	}
	defer store.Close()

	id, err := strconv.ParseInt(os.Args[2], 10, 64)
	if err != nil {
		return err
	}

	relation, err := store.GetRelation(id)
	if err != nil {
		return fmt.Errorf("Failed to get relation: %s\n", err.Error())
	}

	outerParts := [][]int64{}
	innerParts := [][]int64{}
	for _, m := range relation.GetMembers() {
		if m.GetType() == 1 && m.GetRole() == "outer" {
			way, err := store.GetWay(m.GetId())
			if err != nil {
				return err
			}

			outerParts = append(outerParts, way.GetRefs())
		}

		if m.GetType() == 1 && m.GetRole() == "inner" {
			way, err := store.GetWay(m.GetId())
			if err != nil {
				return err
			}

			innerParts = append(innerParts, way.GetRefs())
		}
	}

	outerParts = simplify.Reduce(outerParts)
	innerParts = simplify.Reduce(innerParts)

	outerPolys, err := toGeom(store, outerParts)
	if err != nil {
		return err
	}
	innerPolys, err := toGeom(store, innerParts)
	if err != nil {
		return err
	}

	polygons := make([]*geos.Geometry, 0)
	for _, shell := range outerPolys {
		holes := make([][]geos.Coord, 0)

		if len(innerPolys) > 0 {
			pshell := geos.PrepareGeometry(shell)

			// Find holes
			for i := 0; i < len(innerPolys); i++ {
				hole := innerPolys[i]
				c, err := pshell.Contains(hole)
				if err != nil {
					return err
				}
				if c {
					s, err := hole.Shell()
					if err != nil {
						return err
					}

					c, err := s.Coords()
					if err != nil {
						return err
					}

					holes = append(holes, c)
					innerPolys = append(innerPolys[:i], innerPolys[i+1:]...)
					i-- // Counter-act the increment at the end of the iteration
				}
			}
		}

		s, err := shell.Shell()
		if err != nil {
			return err
		}

		scoords, err := s.Coords()
		if err != nil {
			return err
		}

		polygon, err := geos.NewPolygon(scoords, holes...)
		if err != nil {
			return err
		}
		polygons = append(polygons, polygon)
	}

	var feat *geos.Geometry
	if len(polygons) == 1 {
		feat = polygons[0]
	} else {
		f, err := geos.NewCollection(geos.MULTIPOLYGON, polygons...)
		if err != nil {
			return err
		}
		feat = f
	}

	out, err := geojson.FromGeos(feat)
	if err != nil {
		return err
	}

	b, err := json.Marshal(out)
	if err != nil {
		return err
	}
	os.Stdout.Write(b)

	return nil
}

func toGeom(store *osmtopo.Store, coords [][]int64) ([]*geos.Geometry, error) {
	linestrings := make([]*geos.Geometry, len(coords))
	for i, v := range coords {
		ls, err := expandPoly(store, v)
		if err != nil {
			return nil, err
		}
		linestrings[i] = ls
	}

	return linestrings, nil
}

func expandPoly(store *osmtopo.Store, coords []int64) (*geos.Geometry, error) {
	points := make([]geos.Coord, len(coords))
	for i, c := range coords {
		node, err := store.GetNode(c)
		if err != nil {
			return nil, err
		}
		points[i] = geos.Coord{X: node.GetLon(), Y: node.GetLat()}
	}

	return geos.NewPolygon(points)
}
