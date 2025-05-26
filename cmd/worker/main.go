package main

import (
	"context"
	"job-worker/pkg/config"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {

	// Check if cgroups are available
	if _, err := os.Stat(config.CgroupsBaseDir); os.IsNotExist(err) {
		log.Fatalf("[ERROR] Cgroups not available at %s", config.CgroupsBaseDir)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		log.Println("[INFO] Received shutdown signal, cleaning up and exiting...")

		cancel()
	}()

	<-ctx.Done()

	log.Println("[INFO] Server gracefully stopped")
}
