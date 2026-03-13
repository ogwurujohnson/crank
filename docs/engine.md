## Engine & Workers

This document describes how to configure and run the Crank engine, define workers, register them, and reason about runtime behavior and errors.

All symbols are exposed via `github.com/ogwurujohnson/crank`.

---

## Core Types

### `type Engine`

```go
type Engine struct {
    // created via NewEngine
}
```

Constructed with:

```go
func NewEngine(cfg *Config, broker Broker) (*Engine, error)
```

- **Parameters**
  - `cfg *Config` (**required**): engine configuration (see `docs/configuration.md`).
  - `broker Broker` (**required**): backing broker (typically Redis) used to dequeue, retry, and dead‑letter jobs.
- **Behavior**
  - Sets a default `Logger` on `cfg` if none is provided.
  - Creates an internal worker registry specific to the engine instance.
  - Configures a circuit breaker (`CircuitBreaker`) and a middleware chain composed of:
    - `RecoveryMiddleware`
    - `LoggingMiddleware`
    - `BreakerMiddleware`
  - Instantiates a `Processor` and wires in the circuit breaker.
- **Error behavior**
  - Returns errors from `NewProcessor` when configuration is invalid or the broker cannot be used.

#### Methods

```go
func (e *Engine) Use(middleware ...Middleware)
func (e *Engine) Register(className string, worker Worker)
func (e *Engine) RegisterMany(workers map[string]Worker)
func (e *Engine) Start() error
func (e *Engine) Stop()
```

- **`Use`**
  - Appends one or more `Middleware` instances to the engine’s middleware chain.
  - No‑op if the internal chain is `nil` (should not occur when created via `NewEngine`).
- **`Register`**
  - Registers a single worker instance under the given `className`.
  - Replaces any existing worker registration for that name.
- **`RegisterMany`**
  - Registers multiple workers in a single call using a `map[string]Worker`.
- **`Start`**
  - Starts job fetching, worker goroutines, retry processing, and optional metrics loop.
  - Returns immediately; processing continues in background goroutines.
  - Returns an error only if the processor fails to start; subsequent runtime errors are reported via logs, metrics, and job states rather than by returning from `Start`.
- **`Stop`
  - Signals all background goroutines to stop, waits for them, and logs shutdown.

---

## Workers

### `type Worker`

```go
type Worker interface {
    Perform(ctx context.Context, args ...interface{}) error
}
```

- **Parameters**
  - `ctx context.Context` (**required**): per‑job context with timeout set from configuration. Workers should honor cancellation and timeouts.
  - `args ...interface{}` (**optional**): positional arguments provided when the job was enqueued.
- **Error behavior**
  - Returning `nil` marks the job as successful.
  - Returning a non‑`nil` error:
    - Marks the job as failed for that attempt.
    - Triggers retry logic or moves the job to the dead‑letter queue depending on `Retry` / `RetryCount`.
    - Is logged and emitted as a `JobEvent`.
  - Panics are recovered by `RecoveryMiddleware` and converted into errors.

**Example worker**

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

    // Simulate work; respect ctx for timeouts / cancellation.
    if err := sendEmail(ctx, userID); err != nil {
        return err
    }
    return nil
}
```

---

## Worker Registration APIs

Crank supports two registration mechanisms:

- **Engine‑local registration**, recommended for most applications.
- **Global registration**, useful for libraries or when you do not manage the engine directly.

### Engine‑local registration

```go
func (e *Engine) Register(className string, worker Worker)
func (e *Engine) RegisterMany(workers map[string]Worker)
```

- **Parameters**
  - `className` (**required**): the string class used in `Enqueue` / `EnqueueWithOptions` (e.g. `"EmailWorker"`).
  - `worker` (**required**): a value implementing `Worker`.
- **Behavior**
  - `Register` and `RegisterMany` populate an engine‑specific registry.
  - The engine looks up workers by `job.Class` on each job execution.

**Example**

```go
engine.Register("EmailWorker", EmailWorker{})

engine.RegisterMany(map[string]crank.Worker{
    "ReportWorker": ReportWorker{},
    "CleanupWorker": CleanupWorker{},
})
```

### Global worker registry

```go
func RegisterWorker(className string, worker Worker)
func ListWorkers() []string
```

- **Behavior**
  - `RegisterWorker` stores workers in a global map, protected by a read‑write mutex.
  - `ListWorkers` returns the list of registered class names.
- **When used**
  - The `Processor` first attempts to resolve workers via the engine’s registry.
  - If no engine registry is configured, it falls back to the global registry via `GetWorker`.

**Error behavior**

- When a worker cannot be found:

```go
return fmt.Errorf("worker class '%s' not found", className)
```

- At execution time, this becomes:

```go
return fmt.Errorf("worker not found: %w", err)
```

which then flows through the standard failure and retry pipeline.

---

## Middleware & Handler Chain

### Core types

```go
type Handler func(ctx context.Context, job *payload.Job) error

type Middleware func(next Handler) Handler

type Chain struct {
    middlewares []Middleware
}
```

#### Construction and usage

```go
func NewChain(ms ...Middleware) *Chain
func (c *Chain) Use(m ...Middleware)
func (c *Chain) Wrap(final Handler) Handler
```

- **NewChain**: builds a chain with an initial list of middlewares (outermost last).
- **Use**: appends additional middlewares.
- **Wrap**: applies middlewares around a `final` handler.

### Built‑in middleware

#### `LoggingMiddleware`

```go
func LoggingMiddleware(logger Logger) Middleware
```

- Logs job failures using a redacted view of `job.Args`.
- Uses the current default `Redactor` to convert arguments into a safe string.
- Does not log on success.

#### `RecoveryMiddleware`

```go
func RecoveryMiddleware(logger Logger) Middleware
```

- Catches panics in downstream handlers.
- Logs the panic value, job ID, and stack trace.
- Converts the panic into an `error`:

```go
err = fmt.Errorf("panic: %v", panicValue)
```

#### `BreakerMiddleware`

```go
func BreakerMiddleware(breaker *CircuitBreaker) Middleware
```

- When `breaker` is `nil`, returns a pass‑through middleware.
- When provided, it:
  - Records failures via `breaker.RecordFailure(job.Class)`.
  - Records successes via `breaker.RecordSuccess(job.Class)`.
- The breaker’s state influences the fetcher: when open for a class, jobs are requeued with a short delay instead of being processed.

---

## Runtime Flow & Error Handling

### Job lifecycle within the processor

1. **Fetch**
   - Jobs are fetched from Redis via `broker.Dequeue`.
   - If the breaker is open for the job’s class, the job is requeued and temporarily skipped.
2. **Start event**
   - A `JobEvent` of type `EventJobStarted` is emitted (when metrics are enabled).
3. **Execution**
   - A per‑job `context.Context` with timeout from `Config.GetTimeout()` is created.
   - The middleware chain is applied around a base handler that:
     - Runs the global `Validator` (if any).
     - Resolves the worker.
     - Calls `worker.Perform(ctx, job.Args...)`.
4. **Result**
   - On success:
     - Job state transitions to `success`.
     - Logs a success message including duration.
     - Emits `EventJobSucceeded`.
   - On error (including panic converted by `RecoveryMiddleware`):
     - Job state transitions to `failed`.
     - Logs a failure message including duration and error.
     - Emits `EventJobFailed`.
     - Invokes retry handling:
       - If `RetryCount < Retry`: schedule retry with exponential backoff.
       - Else: mark job as `dead` and move it to the dead‑letter queue.

### Exponential backoff and dead‑letter queue

- On failure:
  - `RetryCount` is incremented.
  - If `RetryCount <= Retry`:
    - Next attempt is scheduled at:

```go
backoff := time.Duration(1<<uint(job.RetryCount)) * time.Second
retryAt := time.Now().Add(backoff)
```

    - The job is added to the retry set via `AddToRetry`.
  - Otherwise:
    - Job state is set to `dead`.
    - Job is added to a dead‑letter set via `AddToDead`.

Jobs in the retry set are periodically scanned and re‑enqueued. If re‑enqueue fails, they are left in the retry set and attempted again later.

### Context cancellation

- When the engine is stopped:
  - The processor’s root context is cancelled.
  - Fetchers and workers exit gracefully.
- Individual jobs receive a `context` with a bounded deadline. Workers should:
  - Check `ctx.Err()` where appropriate.
  - Abort work promptly when `ctx.Done()` is closed.

---

## Example: Engine Setup & Worker Registration

```go
cfg, err := crank.LoadConfig("config/crank.yml")
if err != nil {
    log.Fatalf("failed to load config: %v", err)
}

redis, err := crank.NewRedisClient(cfg.Redis.URL, cfg.Redis.GetNetworkTimeout())
if err != nil {
    log.Fatalf("failed to create redis client: %v", err)
}

engine, err := crank.NewEngine(cfg, redis)
if err != nil {
    log.Fatalf("failed to create engine: %v", err)
}

engine.RegisterMany(map[string]crank.Worker{
    "EmailWorker":  EmailWorker{},
    "ReportWorker": ReportWorker{},
})

// Optionally add custom middleware.
engine.Use(customMiddleware)

if err := engine.Start(); err != nil {
    log.Fatalf("engine failed to start: %v", err)
}

defer engine.Stop()
```

