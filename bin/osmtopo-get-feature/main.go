package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/rubenv/osmtopo"
	"github.com/rubenv/osmtopo/geojson"
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

	feat, err := relation.ToGeometry(store)
	if err != nil {
		return err
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
