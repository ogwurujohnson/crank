## Crank Go SDK

Crank is a Redis‑backed background job processing SDK for Go. It lets your applications enqueue jobs to named queues, run worker processes that execute those jobs concurrently, and observe execution via middleware, validation, and metrics hooks – all from the `github.com/ogwurujohnson/crank` package.

Crank is inspired by mature background job systems (e.g. Sidekiq/Resque) but is designed to feel idiomatic in Go and easy to integrate into existing services.

---

## Installation

Add Crank to your module:

```bash
go get github.com/ogwurujohnson/crank
```

Then import it in your code:

```go
import "github.com/ogwurujohnson/crank"
```

---

## Quick Start

Use the fluent API to create an engine and client from a broker URL and options. For YAML-based configuration, use `crank.QuickStart(configPath)`.

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ogwurujohnson/crank"
)

// EmailWorker is a simple worker that sends an email.
type EmailWorker struct{}

func (w EmailWorker) Perform(ctx context.Context, args ...interface{}) error {
	if len(args) == 0 {
		return fmt.Errorf("missing argument: userID")
	}

	userID, ok := args[0].(string)
	if !ok {
		return fmt.Errorf("expected userID as string, got %T", args[0])
	}

	log.Printf("sending welcome email to user %s", userID)
	return nil
}

func main() {
	brokerURL := os.Getenv("REDIS_URL")
	if brokerURL == "" {
		brokerURL = "redis://localhost:6379/0"
	}

	// 1) Create engine and client with options (or use crank.QuickStart("config/crank.yml") for YAML).
	engine, client, err := crank.New(brokerURL,
		crank.WithConcurrency(10),
		crank.WithTimeout(8*time.Second),
		crank.WithQueues(crank.QueueOption{Name: "default", Weight: 1}),
	)
	if err != nil {
		log.Fatalf("failed to create engine: %v", err)
	}

	crank.SetGlobalClient(client)
	engine.Register("EmailWorker", EmailWorker{})

	// 2) Start processing jobs.
	go func() {
		if err := engine.Start(); err != nil {
			log.Fatalf("engine stopped with error: %v", err)
		}
	}()

	// 3) Enqueue a job using the global helper.
	if _, err := crank.Enqueue("EmailWorker", "default", "user-123"); err != nil {
		log.Printf("failed to enqueue job: %v", err)
	}

	// 4) Wait for shutdown signal.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	engine.Stop()
	log.Println("engine stopped gracefully")
}
```

---

## High‑Level Features

- **Redis‑backed job queues**: Enqueue jobs into named queues with configurable weights and priorities.
- **Typed workers**: Implement the `crank.Worker` interface to define units of work as Go types.
- **Engine orchestration**: `crank.Engine` handles concurrent job fetching, execution, retries, and dead‑letter queues.
- **Config‑driven behavior**: YAML + environment variables configure concurrency, queues, timeouts, and Redis connection details.
- **Flexible enqueueing API**: Use a `*crank.Client` directly or global helpers (`crank.Enqueue`, `crank.EnqueueWithOptions`) after calling `crank.SetGlobalClient`.
- **Retry & backoff**: Automatic exponential backoff with configurable retry counts per job.
- **Dead‑letter queue**: Jobs that exhaust their retry budget are moved to a dead queue for later inspection.
- **Middleware chain**: Compose logging, recovery, circuit breaker, and custom cross‑cutting logic around job execution.
- **Redaction & validation**: Built‑in tools for safe logging of arguments and validating payloads before execution.
- **Metrics & events**: Hook into job lifecycle events via `MetricsHandler` and read aggregate stats via `engine.Stats()`.

---

## Documentation

Detailed API documentation is available in the `docs` folder:

- **Enqueueing jobs** – `docs/enqueueing.md`
- **Engine & workers** – `docs/engine.md`
- **Configuration & Redis broker** – `docs/configuration.md`
- **Advanced topics (validation, redaction, metrics)** – `docs/advanced.md`

### Docs overview

- **`docs/enqueueing.md`**: How to construct clients, enqueue jobs (with and without options), use the global helpers, and handle enqueue‑time errors.
- **`docs/engine.md`**: How the engine works, how to define and register workers, how middleware, retries, dead‑letter queues, and context/timeouts behave.
- **`docs/configuration.md`**: YAML and environment configuration model, default values, and how to set up the Redis broker and wire it into the engine and client.
- **`docs/advanced.md`**: Validator, redactor, circuit breaker, metrics, and stats APIs, with guidance on composing them for robust, observable job processing.

Each document includes method signatures, parameter descriptions, usage examples, and notes on error handling behavior specific to this SDK.

