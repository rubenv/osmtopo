package main

import (
	"fmt"
	"log"
	"os"

	"github.com/kr/pretty"
	"github.com/rubenv/osmtopo"
)

func main() {
	if len(os.Args) != 3 {
		log.Println("Usage: osmtopo-get-relation /path/to/datastore id")
		os.Exit(1)
	}

	store, err := osmtopo.NewStore(os.Args[1])
	if err != nil {
		log.Printf("Failed to open store: %s\n", err.Error())
		os.Exit(1)
	}

	relation, err := store.GetRelation(os.Args[2])
	if err != nil {
		log.Printf("Failed to get relation: %s\n", err.Error())
		os.Exit(1)
	}

	fmt.Printf("%# v\n", pretty.Formatter(relation))

	os.Exit(0)
}
