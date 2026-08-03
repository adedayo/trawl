package main

import (
	"log"
	"os"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "server" {
		log.Println("Starting Trawl Cloud Continuous EASM Server...")
		runServer()
		return
	}
	
	log.Println("Usage: trawl server")
}
