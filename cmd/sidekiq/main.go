package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/quest/sidekiq-go"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "C", "config/sidekiq.yml", "Path to configuration file")
	flag.Parse()

	// Load configuration
	config, err := sidekiq.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Connect to Redis
	redis, err := sidekiq.NewRedisClient(config.Redis.URL, config.Redis.GetNetworkTimeout())
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redis.Close()

	// Initialize global client
	client := sidekiq.NewClient(redis)
	sidekiq.SetGlobalClient(client)

	// Create processor
	processor, err := sidekiq.NewProcessor(config, redis)
	if err != nil {
		log.Fatalf("Failed to create processor: %v", err)
	}

	// Start processor
	if err := processor.Start(); err != nil {
		log.Fatalf("Failed to start processor: %v", err)
	}

	log.Println("Sidekiq started. Press Ctrl+C to stop.")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")
	processor.Stop()
	log.Println("Shutdown complete")
}

