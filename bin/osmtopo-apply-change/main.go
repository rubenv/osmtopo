package main

import (
	"log"
	"os"
	"runtime"

	"github.com/rubenv/osmtopo"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	if len(os.Args) != 3 {
		log.Println("Usage: osmtopo-import /path/to/datastore change.osc.gz")
		os.Exit(1)
	}

	store, err := osmtopo.NewStore(os.Args[1])
	if err != nil {
		log.Printf("Failed to open store: %s\n", err.Error())
		os.Exit(1)
	}

	err = store.ApplyChange(os.Args[2])
	if err != nil {
		log.Printf("Failed to apply changes: %s\n", err.Error())
		os.Exit(1)
	}

	os.Exit(0)
}
