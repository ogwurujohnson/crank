// This example runs the Crank engine with two demo workers. Use it as a reference
// for wiring config, Redis, client, engine, and worker registration.
//
// Run from the repo root:
//
//	go run ./examples/run
//
// Or build and run:
//
//	go build -o crank-example ./examples/run && ./crank-example -C config/crank.yml
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

	engine.RegisterMany(map[string]crank.Worker{
		"EmailWorker":  emailWorker{},
		"ReportWorker": reportWorker{},
	})

	if err := engine.Start(); err != nil {
		log.Fatalf("Failed to start engine: %v", err)
	}

	log.Println("Crank example started. Press Ctrl+C to stop.")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")
	engine.Stop()
	log.Println("Shutdown complete")
}
