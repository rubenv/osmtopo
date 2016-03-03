package main

import (
	"log"
	"os"

	"github.com/rubenv/osmtopo/cmd"
)

func main() {
	err := cmd.Run()
	if err != nil {
		log.Printf(err.Error())
		os.Exit(1)
	}

	os.Exit(0)
}
