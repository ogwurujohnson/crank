# Crank

A high-performance background job processor for Go. Uses a broker abstraction (Redis by default), a managed worker pool, and goroutines for concurrent job processing.

## Features

- **Broker abstraction** — Enqueue, Dequeue, and Ack behind an interface (Redis included; others pluggable)
- **Worker pool manager** — `Processor` runs a configurable pool of workers with graceful shutdown
- **Redis-backed queue** — Weighted queues, automatic retries with exponential backoff, dead job set
- **Web UI** — Monitoring, queue stats, and clear-queue actions
- **Middleware** — Chain hooks around job execution (logging, metrics, redaction)
- **Security** — Optional payload validation, argument redaction for logs, TLS for Redis
- **YAML configuration** — Queues, concurrency, timeouts, Redis URL (and TLS options)
- **Production-ready** — Graceful shutdown (SIGTERM/SIGINT), timeouts, DLQ

## Requirements

- Go 1.21+
- Redis 7.0+ (or compatible: Valkey, Dragonfly)

## Installation

```bash
go get github.com/quest/crank
```

## Architecture

The codebase is organized around a **broker-based** design with a single public package and internal layout:

```
github.com/quest/crank/
├── crank.go                # Public API (re-exports)
├── cmd/crank/              # Optional: standalone worker binary
├── internal/
│   ├── broker/             # Broker interface + Redis implementation
│   ├── config/             # YAML config, queue weights, Redis/TLS options
│   ├── payload/            # Job struct, serialization, validator, redactor
│   └── queue/              # Processor (worker pool), Queue, Worker registry, middleware
├── pkg/sdk/                # Client for enqueueing jobs
└── web/                    # Web UI (stats, queues, clear)
```

- **Producer**: Your app (or `examples/simple_worker`, `examples/web_server`) uses the **Client** to enqueue jobs; the client talks to the **Broker** (e.g. Redis).
- **Broker**: Interface for Enqueue, Dequeue, Ack, retry/dead sets, and stats. Default implementation is **Redis** (`NewRedisClient` / `NewRedisClientWithConfig`).
- **Consumer**: The **Processor** (worker pool manager) pulls jobs from the broker, runs payload validation and middleware, dispatches to registered **Worker** implementations, and handles retries and dead jobs. Run it via `cmd/crank` or by starting the processor inside your own binary.

Data flow:

```
Enqueue:  Client.Enqueue() → Broker.Enqueue() → Redis LPUSH queue:{name}
Process:  Processor → Broker.Dequeue() (BRPOP) → validate → middleware → Worker.Perform()
Failure:  Broker.AddToRetry() (backoff) or Broker.AddToDead() (DLQ)
```

## Quick Start

### 1. Define a worker and enqueue jobs (producer)

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/quest/crank"
)

type EmailWorker struct{}

func (w *EmailWorker) Perform(ctx context.Context, args ...interface{}) error {
	if len(args) < 1 {
		return fmt.Errorf("expected at least 1 argument")
	}
	userID, ok := args[0].(float64) // JSON numbers unmarshal as float64
	if !ok {
		return fmt.Errorf("invalid user ID type")
	}
	fmt.Printf("Sending email to user %.0f\n", userID)
	// Your logic here
	return nil
}

func main() {
	redis, err := crank.NewRedisClient("redis://localhost:6379/0", 5*time.Second)
	if err != nil {
		log.Fatal(err)
	}
	defer redis.Close()

	client := crank.NewClient(redis)
	crank.SetGlobalClient(client)
	crank.RegisterWorker("EmailWorker", &EmailWorker{})

	jid, err := crank.Enqueue("EmailWorker", "default", 123)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Enqueued job: %s\n", jid)
}
```

### 2. Run the worker process (consumer)

Use the standalone worker (loads config and starts the Processor):

```bash
# Ensure Redis is running, then:
go run ./cmd/crank/ -C config/crank.yml
```

Or build and run:

```bash
make build
./bin/crank -C config/crank.yml
```

### 3. Configuration

The library does **not** read any config file automatically. You create a `*crank.Config` by either:

- **`crank.LoadConfig(path)`** — load from a YAML file (any path you choose)
- **Build it in code** — construct `crank.Config` manually and pass it to `NewProcessor`

You do **not** need to provide `config/crank.yml` in your project. That path is just the default used by `cmd/crank` when you run `./bin/crank -C config/crank.yml`. You can use another path, another config format, or no file at all.

Example YAML format (for use with `LoadConfig`):

```yaml
concurrency: 10
queues:
  - [critical, 5]
  - [default, 3]
  - [low, 1]
timeout: 8
verbose: true
redis:
  url: redis://localhost:6379/0
  network_timeout: 5
  # use_tls: true
  # tls_insecure_skip_verify: false  # dev only
```

- `concurrency`: Number of worker goroutines in the pool.
- `queues`: Name and weight; higher weight = more polling share.
- `timeout`: Job execution timeout in seconds.
- `redis.url`: Overridable by `REDIS_URL`. Use `rediss://` or set `use_tls: true` for TLS.

**Config without a file:**

```go
config := &crank.Config{
	Concurrency: 10,
	Timeout:     8,
	Verbose:    true,
	Queues:     []crank.QueueConfig{{Name: "default", Weight: 1}},
	Redis: crank.RedisConfig{
		URL:             os.Getenv("REDIS_URL"),
		NetworkTimeout:  5,
	},
}
processor, _ := crank.NewProcessor(config, broker)
```

## Example usage

### Enqueue with options

```go
jid, err := crank.EnqueueWithOptions("EmailWorker", "critical", &crank.JobOptions{
	Retry:     intPtr(3),
	Backtrace: boolPtr(true),
}, 789)
```

### Web UI (same broker as client)

```go
import (
	"github.com/gorilla/mux"
	"github.com/quest/crank-go/web"
)

router := mux.NewRouter()
web.Mount(router, "/crank", redis) // redis implements crank.Broker
// Visit http://localhost:8080/crank for stats and queue management
```

### Run processor inside your app

```go
config, _ := crank.LoadConfig("config/crank.yml")
broker, _ := crank.NewRedisClient(config.Redis.URL, config.Redis.GetNetworkTimeout())
defer broker.Close()

processor, _ := crank.NewProcessor(config, broker)
processor.Start()
defer processor.Stop()

// Your HTTP server or other work here
```

### Middleware and logging with redaction

```go
crank.AddMiddleware(crank.LoggingMiddleware) // logs failures with redacted args

// Or custom middleware
crank.AddMiddleware(func(ctx context.Context, job *crank.Job, next func() error) error {
	start := time.Now()
	err := next()
	log.Printf("Job %s took %v", job.JID, time.Since(start))
	return err
})
```

### Payload validation and redaction

```go
// Validate job class and arg count (treat payload as untrusted)
crank.SetValidator(crank.ChainValidator{
	crank.SafeClassPattern(),
	crank.MaxArgsCount(10),
})

// Redact sensitive args in logs (default is masking)
crank.SetRedactor(crank.NewFieldMaskingRedactor([]string{"password", "token"}))
```

### Queue and stats

```go
queue := crank.NewQueue("default", broker)
size, _ := queue.Size()
_ = queue.Clear()

stats, _ := crank.GetStats(broker)
fmt.Printf("Processed: %d, Retry: %d, Dead: %d\n", stats.Processed, stats.Retry, stats.Dead)
```

### Custom broker

You can use your own backend by implementing the `crank.Broker` interface. All entry points accept any `Broker`: `NewClient(broker)`, `NewProcessor(config, broker)`, `NewQueue(name, broker)`, `GetStats(broker)`, and `web.Mount(router, path, broker)`.

Implement these methods (job types use `*crank.Job`):

| Method | Purpose |
|--------|--------|
| `Enqueue(queue string, job *Job) error` | Push a job onto the named queue |
| `Dequeue(queues []string, timeout time.Duration) (*Job, string, error)` | Block until a job is available from any of the queues; return job and queue name |
| `Ack(job *Job) error` | Optional: acknowledge after process (Redis uses no-op; at-most-once brokers can use this) |
| `AddToRetry(job *Job, retryAt time.Time) error` | Schedule job for retry at given time |
| `GetRetryJobs(limit int64) ([]*Job, error)` | Return jobs whose retry time has passed |
| `RemoveFromRetry(job *Job) error` | Remove job from retry set (before re-enqueue) |
| `AddToDead(job *Job) error` | Move job to dead set after max retries |
| `GetQueueSize(queue string) (int64, error)` | Queue length for stats/UI |
| `DeleteKey(key string) error` | Delete a key (e.g. for queue clear; key format is `queue:{name}` for the default) |
| `GetStats() (map[string]interface{}, error)` | Return `processed`, `retry`, `dead` (int64), and `queues` (map[string]int64) |
| `Close() error` | Release connections/resources |

Example: plugging a custom broker into the client and processor:

```go
type MyBroker struct { /* your backend client */ }

func (b *MyBroker) Enqueue(queue string, job *crank.Job) error { /* ... */ }
func (b *MyBroker) Dequeue(queues []string, timeout time.Duration) (*crank.Job, string, error) { /* ... */ }
func (b *MyBroker) Ack(job *crank.Job) error { return nil }
func (b *MyBroker) AddToRetry(job *crank.Job, retryAt time.Time) error { /* ... */ }
func (b *MyBroker) GetRetryJobs(limit int64) ([]*crank.Job, error) { /* ... */ }
func (b *MyBroker) RemoveFromRetry(job *crank.Job) error { /* ... */ }
func (b *MyBroker) AddToDead(job *crank.Job) error { /* ... */ }
func (b *MyBroker) GetQueueSize(queue string) (int64, error) { /* ... */ }
func (b *MyBroker) DeleteKey(key string) error { /* ... */ }
func (b *MyBroker) GetStats() (map[string]interface{}, error) { /* ... */ }
func (b *MyBroker) Close() error { return nil }

// Use it like Redis
broker := &MyBroker{}
client := crank.NewClient(broker)
processor, _ := crank.NewProcessor(config, broker)
```

## Examples in this repo

| Example | Description |
|--------|-------------|
| `examples/simple_worker/` | Enqueues jobs; run a worker process separately to consume them. |
| `examples/web_server/` | HTTP server that enqueues jobs and mounts the Crank Web UI. |

Run the simple enqueue example (then start the worker in another terminal):

```bash
go run ./examples/simple_worker/
```

Run the web example (UI at `/crank`, enqueue via `POST /api/jobs?user_id=123`):

```bash
go run ./examples/web_server/
```

## Deployment

See [DEPLOYMENT.md](DEPLOYMENT.md) for systemd, Docker, Kubernetes, and production notes.

## Comparison with Sidekiq (Ruby)

| Feature | Sidekiq (Ruby) | Crank |
|--------|-----------------|------------|
| Language | Ruby | Go |
| Concurrency | Threads | Goroutines + worker pool |
| Broker | Redis | Broker interface (Redis default) |
| Deployment | Ruby stack | Single Go binary |

## License

MIT

## References

- [Sidekiq (Ruby)](https://github.com/sidekiq/sidekiq)
- [Sidekiq wiki](https://github.com/sidekiq/sidekiq/wiki)
