// This example runs the Crank engine with two demo workers. Use it as a reference
// for the fluent API: New(brokerURL, opts...) and optional QuickStart(configPath).
//
// Run from the repo root:
//
//	go run ./examples/run
//
// Or with a config file (QuickStart):
//
//	go run ./examples/run -C config/crank.yml
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

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
	var useConfig bool
	var uiAddr string
	flag.StringVar(&configPath, "C", "config/crank.yml", "Path to configuration file (used if -config is set)")
	flag.BoolVar(&useConfig, "config", false, "Use YAML config file instead of New(brokerURL, opts...)")
	flag.StringVar(&uiAddr, "ui", "", "If set (e.g. :9090), serve the Crank dashboard on this address")
	flag.Parse()

	var engine *crank.Engine
	var client *crank.Client
	var err error

	if useConfig {
		engine, client, err = crank.QuickStart(configPath)
	} else {
		brokerURL := os.Getenv("REDIS_URL")
		if brokerURL == "" {
			brokerURL = "redis://localhost:6379/0"
		}
		engine, client, err = crank.New(brokerURL,
			crank.WithConcurrency(2),
			crank.WithTimeout(10*time.Second),
			crank.WithQueues(
				crank.QueueOption{Name: "default", Weight: 1},
			),
		)
	}
	if err != nil {
		log.Fatalf("Failed to create engine: %v", err)
	}

	crank.SetGlobalClient(client)

	engine.RegisterMany(map[string]crank.Worker{
		"EmailWorker":  emailWorker{},
		"ReportWorker": reportWorker{},
	})

	if err := engine.Start(); err != nil {
		log.Fatalf("Failed to start engine: %v", err)
	}

	rootCtx, stopRoot := context.WithCancel(context.Background())
	defer stopRoot()

	if uiAddr != "" {
		uiSrv := &http.Server{
			Addr:              uiAddr,
			Handler:           engine.DashboardHandler(crank.DashboardOptions{Version: "EXAMPLE"}),
			ReadHeaderTimeout: 10 * time.Second,
		}
		go func() {
			host := uiAddr
			if len(host) > 0 && host[0] == ':' {
				host = "127.0.0.1" + host
			}
			log.Printf("Crank dashboard: http://%s/", host)
			if err := uiSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Printf("dashboard server: %v", err)
			}
		}()
		go func() {
			<-rootCtx.Done()
			shCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = uiSrv.Shutdown(shCtx)
		}()
	}

	log.Println("Crank example started. Press Ctrl+C to stop.")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")
	stopRoot()
	engine.Stop()
	log.Println("Shutdown complete")
}
