package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/ogwurujohnson/crank"
)

// Demo workers (so enqueued jobs from examples/simple_worker can be processed)
type emailWorker struct{}

func (emailWorker) Perform(ctx context.Context, args ...interface{}) error {
	if len(args) < 1 {
		return fmt.Errorf("expected at least 1 argument")
	}
	log.Printf("EmailWorker: sent to user %v", args[0])
	return nil
}

type reportWorker struct{}

func (reportWorker) Perform(ctx context.Context, args ...interface{}) error {
	if len(args) < 1 {
		return fmt.Errorf("expected at least 1 argument")
	}
	log.Printf("ReportWorker: report %v", args[0])
	return nil
}

func main() {
	var configPath string
	flag.StringVar(&configPath, "C", "config/crank.yml", "Path to configuration file")
	flag.Parse()

	config, err := crank.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

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

	client := crank.NewClient(redis)
	crank.SetGlobalClient(client)

	engine, err := crank.NewEngine(config, redis)
	if err != nil {
		log.Fatalf("Failed to create engine: %v", err)
	}

	engine.RegisterWorkers(map[string]crank.Worker{
		"EmailWorker":  emailWorker{},
		"ReportWorker": reportWorker{},
	})

	if err := engine.Start(); err != nil {
		log.Fatalf("Failed to start engine: %v", err)
	}

	log.Println("Crank started. Press Ctrl+C to stop.")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")
	engine.Stop()
	log.Println("Shutdown complete")
}
