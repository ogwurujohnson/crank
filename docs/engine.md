# Engine & Workers

This document describes how to create and run the Crank engine, define workers, register them, and reason about runtime behavior and errors. All symbols are exposed via `github.com/ogwurujohnson/crank`.

---

## Creating the Engine

You create an engine and client in one of two ways: **programmatically** with `New(brokerURL, opts...)` or **from a YAML file** with `QuickStart(configPath)`.

### `func New`

```go
func New(brokerURL string, opts ...Option) (*Engine, *Client, error)
```

- **Parameters**
  - `brokerURL` (**required**): backend URL (e.g. `redis://localhost:6379/0`). The broker type is inferred from the URL scheme unless overridden with `WithBroker("redis")` (or `nats` / `rabbitmq` when supported).
  - `opts`: optional functional options (e.g. `WithConcurrency`, `WithTimeout`, `WithQueues`, `WithLogger`).
- **Returns**
  - `*Engine`: the job processor; register workers and call `Start()` / `Stop()`.
  - `*Client`: use to enqueue jobs; you may call `SetGlobalClient(client)` so global `Enqueue` / `EnqueueWithOptions` work.
  - `error`: non-nil if the broker cannot be created or the engine fails to initialize.

**Example**

```go
engine, client, err := crank.New("redis://localhost:6379/0",
    crank.WithConcurrency(10),
    crank.WithTimeout(8*time.Second),
    crank.WithQueues(
        crank.QueueOption{Name: "default", Weight: 1},
        crank.QueueOption{Name: "critical", Weight: 5},
    ),
)
if err != nil {
    log.Fatalf("New: %v", err)
}
defer func() { _ = engine.Stats() } // optional: read stats
crank.SetGlobalClient(client)
```

### `func QuickStart`

```go
func QuickStart(configPath string) (*Engine, *Client, error)
```

- **Parameters**
  - `configPath`: path to a YAML configuration file (see `docs/configuration.md`). The file must specify `broker` (e.g. `redis`) and the matching section (e.g. `redis.url`, `redis.network_timeout`, etc.).
- **Behavior**
  - Loads config from the file, creates the broker (Redis today) and engine, then the client. Calls `SetGlobalClient(client)` so global enqueue helpers work.
- **Returns**
  - Same as `New`: `(*Engine, *Client, error)`.

**Example**

```go
engine, client, err := crank.QuickStart("config/crank.yml")
if err != nil {
    log.Fatalf("QuickStart: %v", err)
}
// client is already set as global
engine.RegisterMany(map[string]crank.Worker{
    "EmailWorker":  EmailWorker{},
    "ReportWorker": ReportWorker{},
})
if err := engine.Start(); err != nil {
    log.Fatalf("Start: %v", err)
}
defer engine.Stop()
```

### `func NewTestEngine`

```go
func NewTestEngine(opts ...Option) (*Engine, *Client, *TestBroker, error)
```

- **Behavior**
  - Builds an engine and client backed by an **in-memory broker**. No Redis or other external broker is required. Intended for tests.
- **Returns**
  - `*Engine`, `*Client`: same as `New`.
  - `*TestBroker`: use `RetryJobs()`, `DeadJobs()`, and `GetEnqueuedJobs(queue)` to assert on job state in tests.
- **Example**: see `crank_test.go` in the repo.

---

## Core Type: Engine

```go
type Engine struct {
    // opaque
}
```

Constructed only via `New`, `QuickStart`, or `NewTestEngine`.

### Methods

```go
func (e *Engine) Use(middleware ...Middleware)
func (e *Engine) Register(className string, worker Worker)
func (e *Engine) RegisterMany(workers map[string]Worker)
func (e *Engine) Start() error
func (e *Engine) Stop()
func (e *Engine) Stats() (*Stats, error)
```

- **Use**
  - Appends one or more middlewares to the engine’s chain (e.g. custom logging or metrics).
- **Register / RegisterMany**
  - Register workers under a class name. The class name must match the first argument to `Enqueue(workerClass, queue, args...)`. Replaces any existing registration for that name.
- **Start**
  - Starts job fetching, worker goroutines, retry loop, and optional metrics loop. Returns immediately; processing runs in the background. Returns an error only if startup fails.
- **Stop**
  - Signals all goroutines to stop and waits for them. Safe to call after `Start()`.
- **Stats**
  - Returns queue statistics (processed count, retry count, dead count, per-queue sizes). Use this instead of accessing a broker directly.

---

## Workers

### `type Worker`

```go
type Worker interface {
    Perform(ctx context.Context, args ...interface{}) error
}
```

- **ctx**: per-job context with timeout from configuration. Workers should respect cancellation and deadlines.
- **args**: the variadic arguments passed when the job was enqueued (e.g. `Enqueue("EmailWorker", "default", "user-123")` → `args` = `[]interface{}{"user-123"}`).
- **Return**
  - `nil`: job is considered successful.
  - Non-nil `error`: job is failed for this attempt; triggers retry or move to dead queue depending on retry count.

**Example**

```go
type EmailWorker struct{}

func (w EmailWorker) Perform(ctx context.Context, args ...interface{}) error {
    if len(args) < 1 {
        return fmt.Errorf("missing userID argument")
    }
    userID, ok := args[0].(string)
    if !ok {
        return fmt.Errorf("expected userID as string, got %T", args[0])
    }
    return sendEmail(ctx, userID)
}
```

Registration:

```go
engine.Register("EmailWorker", EmailWorker{})
engine.RegisterMany(map[string]crank.Worker{
    "ReportWorker": ReportWorker{},
    "CleanupWorker": CleanupWorker{},
})
```

---

## Global worker registry (optional)

```go
func RegisterWorker(className string, worker Worker)
func ListWorkers() []string
```

- The engine first looks up workers in its **own** registry (from `Register` / `RegisterMany`). If not found, it falls back to the **global** registry.
- Use the global registry when you do not hold a reference to the engine (e.g. in libraries). Prefer engine-local registration for most applications.

---

## Middleware & handler chain

The engine wraps job execution in a chain of middlewares. Built-in: recovery (panic → error), logging (failed jobs + redacted args), circuit breaker (per class).

### Types

```go
type Handler func(ctx context.Context, job *Job) error
type Middleware func(next Handler) Handler
type Chain struct { ... }
```

### Adding middleware

```go
engine.Use(myMiddleware)
```

Built-in middleware (already in the chain):

- **RecoveryMiddleware(logger)** – catches panics, logs stack, converts to error.
- **LoggingMiddleware(logger)** – logs job failures with redacted args (see redactor in `docs/advanced.md`).
- **BreakerMiddleware(breaker)** – records success/failure per job class; used internally by the engine.

---

## Runtime flow & errors

1. **Fetch** – Jobs are dequeued from the broker (by queue weight). If the circuit breaker is open for the job’s class, the job may be requeued and skipped temporarily.
2. **Start** – A `JobEvent` of type `EventJobStarted` can be emitted when metrics are configured.
3. **Execute** – A context with timeout (from config) is created. The chain runs: validator (if set) → worker lookup → `worker.Perform(ctx, job.Args...)`.
4. **Result**
   - **Success**: job state → success, log, emit `EventJobSucceeded`.
   - **Error (or panic)**: job state → failed, log, emit `EventJobFailed`, then:
     - If `RetryCount < Retry`: schedule retry with exponential backoff (`2^RetryCount` seconds), add to retry set.
     - Else: set state to dead, add to dead set.

Jobs in the retry set are periodically re-enqueued. When the engine is stopped, the root context is cancelled and fetchers/workers exit.

---

## Example: full setup

```go
engine, client, err := crank.New("redis://localhost:6379/0",
    crank.WithConcurrency(10),
    crank.WithTimeout(8*time.Second),
    crank.WithQueues(crank.QueueOption{Name: "default", Weight: 1}),
)
if err != nil {
    log.Fatalf("New: %v", err)
}

crank.SetGlobalClient(client)
engine.RegisterMany(map[string]crank.Worker{
    "EmailWorker":  EmailWorker{},
    "ReportWorker": ReportWorker{},
})

if err := engine.Start(); err != nil {
    log.Fatalf("Start: %v", err)
}
defer engine.Stop()

// Enqueue from anywhere
jid, err := crank.Enqueue("EmailWorker", "default", "user-123")
```
