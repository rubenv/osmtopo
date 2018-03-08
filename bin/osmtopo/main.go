package main

import (
	"log"

	"github.com/rubenv/osmtopo/bin/osmtopo/cmd"
)

func main() {
	err := cmd.Run()
	if err != nil {
		log.Fatal(err.Error())
	}
}
