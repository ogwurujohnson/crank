## Architecture and internals

This page dives into how Crank is structured under the hood: the broker abstraction, processor and worker pool, backpressure, middleware, payload validation, and the web UI.

### High-level architecture

The codebase is organized around a broker-based design with a single public package and internal implementation:

```text
github.com/ogwurujohnson/crank/
├── crank.go                # Public API (re-exports)
├── engine.go               # Engine (processor + worker registry)
├── cmd/crank/              # Standalone worker binary
├── internal/
│   ├── broker/             # Broker interface + Redis implementation
│   ├── config/             # YAML config, queue weights, Redis/TLS options
│   ├── payload/            # Job struct, serialization, validator, redactor
│   └── queue/              # Processor (worker pool), Queue, Worker registry, middleware
├── pkg/sdk/                # Client for enqueueing jobs
└── web/                    # Web UI (stats, queues, clear)
```

- **Producer**: Your application (or examples) uses the client to enqueue jobs.
- **Broker**: Implements enqueue/dequeue, retry/dead sets, and stats.
- **Consumer**: The processor pulls jobs from the broker and dispatches them to workers.

### Broker abstraction

Crank talks to a `Broker` interface. The default implementation is Redis, but any backend can be plugged in by implementing the same contract.

Key responsibilities:

- Enqueue jobs onto named queues.
- Dequeue jobs from one of several queues with a blocking timeout.
- Manage retry and dead sets (DLQ).
- Provide queue sizes and top-level stats (`processed`, `retry`, `dead`).

Data flow through the broker:

```text
Enqueue:  Client.Enqueue() → Broker.Enqueue() → Redis LPUSH queue:{name}
Process:  Fetcher → Broker.Dequeue() (BRPOP) → jobCh (blocks if all workers busy) → Worker → validate → middleware → Worker.Perform()
Failure:  Broker.AddToRetry() (backoff) or Broker.AddToDead() (DLQ)
```

Because everything depends only on `Broker`, you can:

- Use Redis for production and a different broker in tests.
- Swap in a custom backend (SQL, other queues) without changing engine or client code.

### Processor, worker pool, and backpressure

The processor is responsible for executing jobs with controlled concurrency.

Components:

- **Fetcher goroutine**: Calls `Broker.Dequeue` in a loop and sends jobs into an unbuffered channel.
- **Worker goroutines**: `N` workers (where \(N =\) `config.Concurrency`) read from the channel and execute jobs.

Backpressure is achieved by using an unbuffered job channel:

- When all workers are busy, the channel is full.
- The fetcher blocks on `jobCh <- job` and thus stops pulling from the broker.
- As soon as a worker finishes and reads from the channel, the fetcher can pull again.

This design ensures Crank does not overload your workers or Redis when jobs are slow or spiky.

### Engine and worker registry

The `Engine` owns:

- The `config.Config` and chosen `Broker`.
- The `Processor` (fetcher + worker pool).
- A registry mapping worker class names (`string`) to concrete `Worker` implementations.
- The middleware chain used for job execution.

Workers are registered by name:

```go
engine.Register("EmailWorker", &EmailWorker{})
// or
engine.RegisterMany(map[string]crank.Worker{
	"EmailWorker": &EmailWorker{},
})
```

At runtime, jobs reference the worker by its class name; the engine looks up the worker and calls its `Perform` method.

### Middleware onion

Middleware wraps job execution using an onion/decorator pattern:

- `Handler`: `func(ctx context.Context, job *crank.Job) error`
- `Middleware`: `func(next Handler) Handler`

The engine composes the middleware stack once at startup, producing a single baked handler for all jobs. This avoids per-job allocation or recomposition.

Example:

```go
engine, _ := crank.NewEngine(config, broker)

engine.Use(crank.LoggingMiddleware(config.Logger))
engine.Use(func(next crank.Handler) crank.Handler {
	return func(ctx context.Context, job *crank.Job) error {
		start := time.Now()
		err := next(ctx, job)
		config.Logger.Info("job timing", "jid", job.JID, "duration", time.Since(start))
		return err
	}
})
```

Built-in middleware:

- `LoggingMiddleware(logger)`: logs job failures with redacted arguments.
- `RecoveryMiddleware(logger)`: catches panics, logs a stack trace, and converts the panic into an error so the job can be retried or moved to dead.

### Payload validation and redaction

Crank treats job payloads as untrusted. You can plug in validators and redactors to protect your system and logs.

Validators:

- Ensure worker class names are safe and expected.
- Enforce argument count or shape limits.

Example:

```go
crank.SetValidator(crank.ChainValidator{
	crank.SafeClassPattern(),
	crank.MaxArgsCount(10),
})
```

Redactors:

- Mask or remove sensitive fields from logs (e.g. `password`, `token`).

Example:

```go
crank.SetRedactor(crank.NewFieldMaskingRedactor([]string{"password", "token"}))
```

### Web UI

The `web` package exposes a small UI and JSON API for inspecting queues, retries, and dead jobs, and for clearing queues.

Mount it on your router:

```go
import (
	"github.com/gorilla/mux"
	"github.com/ogwurujohnson/crank/web"
)

router := mux.NewRouter()
web.Mount(router, "/crank", redis) // redis implements crank.Broker
```

Endpoints under the mount path:

- `/` – Dashboard (stats + queues).
- `/stats` – JSON stats (`processed`, `retry`, `dead`, `queues`).
- `/queues` – JSON queue size map.
- `/retries` – JSON retry set.
- `/dead` – JSON dead set.
- `/queues/{queue}/clear` – POST to clear a queue (destructive).

In production, protect the UI with authentication, CSRF protection, or network-level controls before exposing destructive endpoints.

### Custom brokers

To integrate another storage broker, implement the `Broker` interface:

```go
type MyBroker struct { /* your backend client */ }

func (b *MyBroker) Enqueue(queue string, job *crank.Job) error                     { /* ... */ }
func (b *MyBroker) Dequeue(queues []string, timeout time.Duration) (*crank.Job, string, error) {
	/* ... */
}
func (b *MyBroker) Ack(job *crank.Job) error                                      { return nil }
func (b *MyBroker) AddToRetry(job *crank.Job, retryAt time.Time) error            { /* ... */ }
func (b *MyBroker) GetRetryJobs(limit int64) ([]*crank.Job, error)                { /* ... */ }
func (b *MyBroker) RemoveFromRetry(job *crank.Job) error                          { /* ... */ }
func (b *MyBroker) AddToDead(job *crank.Job) error                                { /* ... */ }
func (b *MyBroker) GetDeadJobs(limit int64) ([]*crank.Job, error)                 { /* ... */ }
func (b *MyBroker) GetQueueSize(queue string) (int64, error)                      { /* ... */ }
func (b *MyBroker) DeleteKey(key string) error                                    { /* ... */ }
func (b *MyBroker) GetStats() (map[string]interface{}, error)                     { /* ... */ }
func (b *MyBroker) Close() error                                                  { return nil }
```

Once implemented, plug it into the client and engine:

```go
broker := &MyBroker{}
client := crank.NewClient(broker)
engine, _ := crank.NewEngine(config, broker)
```

### Where to go next

- For basic setup and usage, see [Getting started](getting-started.md).
- For configuration knobs, logging, metrics, TLS, and tuning, see [Advanced configuration](advanced-configuration.md).

