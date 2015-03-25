package main

import (
	"fmt"
	"log"
	"os"
	"runtime"

	"github.com/rubenv/osmtopo"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	if len(os.Args) != 2 {
		log.Println("Usage: osmtopo-reindex /path/to/datastore")
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
		os.Exit(1)
	}
	defer store.Close()

	err = store.Reindex()
	if err != nil {
		return fmt.Errorf("Failed to apply changes: %s\n", err.Error())
		os.Exit(1)
	}

	return nil
}
