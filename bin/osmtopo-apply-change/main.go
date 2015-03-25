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

	if len(os.Args) != 3 {
		log.Println("Usage: osmtopo-import /path/to/datastore change.osc.gz")
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

	err = store.ApplyChange(os.Args[2])
	if err != nil {
		return fmt.Errorf("Failed to apply changes: %s\n", err.Error())
		os.Exit(1)
	}

	return nil
}
