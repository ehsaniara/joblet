package main

import (
	"fmt"
	"log"
	"os"
	"worker/internal/modes"
)

func main() {
	mode := os.Getenv("WORKER_MODE")

	var err error
	switch mode {
	case "server":
		err = modes.RunServer()
	case "init":
		err = modes.RunJobInit()
	default:
		err = fmt.Errorf("unknown mode: %s", mode)
	}

	if err != nil {
		log.Fatalf("Worker failed: %v", err)
	}
}
