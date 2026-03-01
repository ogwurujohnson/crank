package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/quest/crank"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "C", "config/crank.yml", "Path to configuration file")
	flag.Parse()

	// Load configuration
	config, err := crank.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Connect to Redis (with optional TLS from config)
	redis, err := crank.NewRedisClientWithConfig(crank.RedisBrokerConfig{
		URL:                   config.Redis.URL,
		Timeout:               config.Redis.GetNetworkTimeout(),
		UseTLS:                config.Redis.UseTLS,
		TLSInsecureSkipVerify: config.Redis.TLSInsecureSkipVerify,
	})
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redis.Close()

	// Initialize global client
	client := crank.NewClient(redis)
	crank.SetGlobalClient(client)

	// Create processor
	processor, err := crank.NewProcessor(config, redis)
	if err != nil {
		log.Fatalf("Failed to create processor: %v", err)
	}

	// Start processor
	if err := processor.Start(); err != nil {
		log.Fatalf("Failed to start processor: %v", err)
	}

	log.Println("Crank started. Press Ctrl+C to stop.")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")
	processor.Stop()
	log.Println("Shutdown complete")
}
