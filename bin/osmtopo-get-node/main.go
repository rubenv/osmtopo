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
		log.Println("Usage: osmtopo-get-node /path/to/datastore id")
		os.Exit(1)
	}

	store, err := osmtopo.NewStore(os.Args[1])
	if err != nil {
		log.Printf("Failed to open store: %s\n", err.Error())
		os.Exit(1)
	}

	node, err := store.GetNode(os.Args[2])
	if err != nil {
		log.Printf("Failed to get node: %s\n", err.Error())
		os.Exit(1)
	}

	fmt.Printf("%# v\n", pretty.Formatter(node))

	os.Exit(0)
}
