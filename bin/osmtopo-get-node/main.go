package main

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/kr/pretty"
	"github.com/rubenv/osmtopo"
)

func main() {
	if len(os.Args) != 3 {
		log.Println("Usage: osmtopo-get-node /path/to/datastore id")
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

	node, err := store.GetNode(id)
	if err != nil {
		return fmt.Errorf("Failed to get node: %s\n", err.Error())
	}

	fmt.Printf("%# v\n", pretty.Formatter(node))
	return nil
}
