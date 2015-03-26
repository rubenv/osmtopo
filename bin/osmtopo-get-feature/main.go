package main

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/kr/pretty"
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

	relation, err := store.GetRelation(os.Args[2])
	if err != nil {
		return fmt.Errorf("Failed to get relation: %s\n", err.Error())
	}

	outerParts := [][]int64{}
	for _, m := range relation.GetMembers() {
		if m.GetType() == 1 && m.GetRole() == "outer" {
			way, err := store.GetWay(strconv.FormatInt(m.GetId(), 10))
			if err != nil {
				return err
			}

			outerParts = append(outerParts, way.GetRefs())

			/*
				for _, id := range way.GetRefs() {
					node, err := store.GetNode(strconv.FormatInt(id, 10))
					if err != nil {
						return err
					}

					fmt.Println(node)
				}
			*/
		}

		if m.GetType() == 1 && m.GetRole() == "inner" {
			panic("Don't understand inner rings yet!")
		}
	}

	fmt.Printf("%# v\n", pretty.Formatter(simplify.Reduce(outerParts)))

	return nil
}
