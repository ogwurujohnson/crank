# Crank

<p>
  <a href="https://github.com/ogwurujohnson/crank/releases"><img src="https://img.shields.io/github/release/ogwurujohnson/crank.svg" alt="Latest Release"></a>
  <a href="https://pkg.go.dev/github.com/ogwurujohnson/crank?tab=doc"><img src="https://godoc.org/github.com/golang/gddo?status.svg" alt="Go Docs"></a>
  <a href="https://github.com/ogwurujohnson/crank/actions/workflows/ci.yml"><img src="https://github.com/ogwurujohnson/crank/workflows/ci/badge.svg" alt="Build Status"></a>
</p>

Crank is a background job processing SDK for Go. It lets your applications enqueue jobs to named queues, run worker processes that execute those jobs concurrently, and observe execution via middleware, validation, and metrics hooks. You use a single package: `github.com/ogwurujohnson/crank`.

Broker backends are pluggable: **Redis** is supported today; **NATS** and **RabbitMQ** are reserved for future implementations. You choose the broker via configuration (`broker: redis` in YAML) or by passing a URL to `New()` (e.g. `redis://localhost:6379/0`). The SDK is inspired by systems like Sidekiq/Resque but is designed to feel idiomatic in Go.

---

## Installation

```bash
go get github.com/ogwurujohnson/crank
```

```go
import "github.com/ogwurujohnson/crank"
```

---

## Quick Start

Create an engine and client with the fluent API, or use a YAML config file.

**Programmatic (recommended)**

```go
engine, client, err := crank.New("redis://localhost:6379/0",
    crank.WithConcurrency(10),
    crank.WithTimeout(8*time.Second),
    crank.WithQueues(crank.QueueOption{Name: "default", Weight: 1}),
)
if err != nil {
    log.Fatalf("failed to create engine: %v", err)
}
defer engine.Stop()

crank.SetGlobalClient(client)
engine.Register("EmailWorker", EmailWorker{})

if err := engine.Start(); err != nil {
    log.Fatalf("engine start: %v", err)
}

// Enqueue jobs
jid, _ := crank.Enqueue("EmailWorker", "default", "user-123")
```

**From YAML config**

```go
engine, client, err := crank.QuickStart("config/crank.yml")
if err != nil {
    log.Fatalf("QuickStart: %v", err)
}
// QuickStart already calls SetGlobalClient(client)
engine.Register("EmailWorker", EmailWorker{})
engine.Start()
```

---

## Example runner

The repo includes an example that runs two demo workers (EmailWorker, ReportWorker). From the **repo root**:

| Command | Description |
|--------|-------------|
| `go run ./examples/run` | Run with **fluent API**: uses `REDIS_URL` (default `redis://localhost:6379/0`), concurrency 2, timeout 10s, queue `default`. |
| `go run ./examples/run -config` | Run with **YAML config**: loads `config/crank.yml` (see `-C` to change path). |
| `go run ./examples/run -config -C path/to/crank.yml` | Same as `-config` but use a custom config file. |

**Flags**

- **`-config`**  
  Use YAML configuration instead of the fluent API. If not set, the example uses `crank.New(brokerURL, opts...)` with defaults.

- **`-C`**  
  Path to the YAML config file. Used only when `-config` is set. Default: `config/crank.yml`.

**Build and run a binary**

```bash
go build -o crank-example ./examples/run
./crank-example                    # fluent API, default Redis URL
./crank-example -config            # config file
./crank-example -config -C my.yml  # custom config path
```

---

## Testing without Redis

Use the in-memory broker for database-free tests:

```go
engine, client, tb, err := crank.NewTestEngine(
    crank.WithConcurrency(2),
    crank.WithTimeout(5*time.Second),
)
if err != nil {
    t.Fatalf("NewTestEngine: %v", err)
}

engine.Register("MyWorker", myWorker{})
engine.Start()
defer engine.Stop()

client.Enqueue("MyWorker", "default", "arg1")
// ... run job ...

// Inspect retry/dead/enqueued state
retry := tb.RetryJobs()
dead := tb.DeadJobs()
enqueued := tb.GetEnqueuedJobs("default")
```

See `crank_test.go` in the repo for full examples.

---

## Features

- **Pluggable brokers**: Redis today; broker chosen by config (`broker: redis`) or URL scheme. NATS/RabbitMQ reserved for future use.
- **Fluent API**: `New(brokerURL, opts...)` with `WithConcurrency`, `WithTimeout`, `WithQueues`, `WithLogger`, `WithBroker`, etc.
- **YAML config**: `QuickStart(path)` loads `broker`, `broker_url`, `redis`/`nats` sections, queues, timeouts, concurrency.
- **Workers**: Implement `crank.Worker` (e.g. `Perform(ctx, args...) error`); register with `engine.Register` or `engine.RegisterMany`.
- **Queues**: Named queues with weights; engine polls by weight. Default queue: `default`.
- **Retries & dead queue**: Exponential backoff; configurable retry count per job; jobs that exhaust retries move to a dead set.
- **Middleware**: Built-in recovery, logging, circuit breaker; add more with `engine.Use(middleware)`.
- **Validation & redaction**: Optional global validator and redactor for job args (see `docs/advanced.md`).
- **Stats**: `engine.Stats()` returns processed, retry, dead, and per-queue sizes.
- **Global client**: `SetGlobalClient(client)` then `crank.Enqueue(...)` / `crank.EnqueueWithOptions(...)` from anywhere.

---

## Documentation

| Document | Description |
|----------|-------------|
| [docs/engine.md](docs/engine.md) | Engine, workers, registration, middleware, retries, and lifecycle. |
| [docs/enqueueing.md](docs/enqueueing.md) | Client, `Enqueue` / `EnqueueWithOptions`, global helpers, `Job` and `JobOptions`. |
| [docs/configuration.md](docs/configuration.md) | YAML config: `broker`, `broker_url`, `redis`, `nats`, queues, timeouts. |
| [docs/advanced.md](docs/advanced.md) | Validation, redaction, circuit breaker, metrics events, and stats. |
| [SECURITY.md](SECURITY.md) | Security considerations: config path, TLS, redaction, queue names, reporting. |

All public types and functions live in `github.com/ogwurujohnson/crank`.

**Maintainer:** ogwurujohnson@gmail.com
