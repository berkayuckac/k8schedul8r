package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/berkayuckac/k8schedul8r/pkg/config"
	"github.com/berkayuckac/k8schedul8r/pkg/scheduler"
)

func main() {
	configPath := flag.String("config", "", "Path to configuration file")                               // TODO: Make this configurable
	pollInterval := flag.Duration("interval", 30*time.Second, "How often to check for scaling changes") // TODO: Make this configurable
	flag.Parse()

	if *configPath == "" {
		log.Fatal("--config flag is required")
	}

	// Create configuration provider
	provider := config.NewLocalProvider(*configPath) // TODO: Ext. API provider option on the start

	// Create the scheduler
	sched, err := scheduler.New(provider, scheduler.Options{
		PollInterval: *pollInterval,
	})
	if err != nil {
		log.Fatalf("Failed to create scheduler: %v", err)
	}

	// Setup shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		log.Printf("Received signal %v, initiating shutdown", sig)
		cancel()
		// Ensure scheduler is stopped gracefully
		sched.Stop()
	}()

	// Start the scheduler
	if err := sched.Start(ctx); err != nil {
		log.Fatalf("Scheduler failed: %v", err)
	}
}
