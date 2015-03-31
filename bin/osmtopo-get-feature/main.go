package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/rubenv/osmtopo"
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

	if len(innerParts) > 0 {
		panic("No support for inner rings yet!")
	}

	properties := map[string]string{}
	name, exists := relation.GetTag("name")
	if exists {
		properties["name"] = name
	}

	feat := &Feature{
		Id:         relation.GetId(),
		Type:       "Feature",
		Properties: properties,
	}

	if len(outerParts) == 1 {
		c, err := expandPoly(store, outerParts[0])
		if err != nil {
			return err
		}

		feat.Geometry = &Polygon{
			Geometry: Geometry{
				Type: "Polygon",
			},
			Coordinates: [][]Coordinate{c},
		}
	} else {
		panic("Multiple polygons")
	}

	fc := &FeatureCollection{
		Type:     "FeatureCollection",
		Features: []*Feature{feat},
	}

	b, err := json.Marshal(fc)
	if err != nil {
		return err
	}
	os.Stdout.Write(b)

	return nil
}

func expandPoly(store *osmtopo.Store, coords []int64) ([]Coordinate, error) {
	result := make([]Coordinate, len(coords))
	for i, c := range coords {
		node, err := store.GetNode(c)
		if err != nil {
			return nil, err
		}
		result[i][0] = node.GetLon()
		result[i][1] = node.GetLat()
	}
	return result, nil
}

type FeatureCollection struct {
	Type     string     `json:"type"`
	Features []*Feature `json:"features"`
}

type Feature struct {
	Id         int64             `json:"id"`
	Type       string            `json:"type"`
	Properties map[string]string `json:"properties"`
	Geometry   interface{}       `json:"geometry"`
}

type Geometry struct {
	Type string `json:"type"`
}

type Polygon struct {
	Geometry
	Coordinates [][]Coordinate `json:"coordinates"`
}

type Coordinate [2]float64
