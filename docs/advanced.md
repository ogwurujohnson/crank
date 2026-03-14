## Advanced Topics

This document covers advanced features of the Crank SDK: validation, redaction, circuit breaking, metrics, and job statistics. It also summarizes error‑handling patterns that are specific to these features.

All symbols are exposed via `github.com/ogwurujohnson/crank`.

---

## Validation

Validation allows you to enforce constraints on jobs before they are executed. The processor consults the global validator (if configured) prior to worker lookup and execution.

### Types

```go
type Validator interface {
    Validate(job *Job) error
}

type ChainValidator []Validator
```

Crank also provides a functional adapter:

```go
type ValidatorFunc func(job *Job) error
```

with:

```go
func (f ValidatorFunc) Validate(job *Job) error
```

### Global validator API

```go
func SetValidator(v Validator)
func GetValidator() Validator
```

- **Behavior**
  - `SetValidator` installs a global validator used by all processors.
  - `GetValidator` returns the current global validator (or `nil` if unset).

### Built‑in validators

Crank provides several helpers that construct `Validator` instances:

```go
func MaxArgsCount(maxArgs int) Validator
func ClassAllowlist(classes map[string]bool) Validator
func ClassPattern(pattern *regexp.Regexp) Validator
func MaxPayloadSize(maxBytes int) Validator
```

and a convenience function that builds a “safe class name” pattern:

```go
func SafeClassPattern() Validator
```

#### Behavior overview

- **MaxArgsCount**
  - Fails when `len(job.Args) > maxArgs`.
  - Error message: `job args count %d exceeds max %d`.
- **ClassAllowlist**
  - Fails when `job.Class` is not present or false in the provided map.
  - Error message: `job class '%s' not in allowlist`.
- **ClassPattern**
  - Fails when `pattern.MatchString(job.Class)` is false.
  - Error message: `job class '%s' does not match allowed pattern`.
- **MaxPayloadSize**
  - Serializes the job to JSON and checks the byte length.
  - Fails when the serialized job exceeds `maxBytes`.
  - Error message: `job payload size %d exceeds max %d bytes`.
- **ChainValidator**
  - Applies validators in sequence, returning the first error encountered.

**Example: configuring validators**

```go
validators := crank.ChainValidator{
    crank.MaxArgsCount(5),
    crank.SafeClassPattern(), // only [A-Za-z0-9_]
}

crank.SetValidator(validators)
```

**Error behavior**

- When a validator fails for a job:
  - The job is not dispatched to a worker.
  - The error is returned from the handler, causing the standard failure path:
    - Job marked as failed for that attempt.
    - Logged and forwarded to retry / dead‑letter logic.

---

## Redaction

Redaction controls how job arguments are logged. When jobs fail, the logging middleware uses the current redactor to produce a safe, redacted representation of `job.Args`.

### Types

```go
type Redactor interface {
    RedactArgs(args []interface{}) string
}
```

Built‑in implementations:

```go
type NoopRedactor struct{}
type MaskingRedactor struct{}

type FieldMaskingRedactor struct {
    Keys []string
}
```

### Global redactor API

```go
func SetRedactor(r Redactor)
func GetRedactor() Redactor
```

- **Behavior**
  - `SetRedactor` replaces the default redactor.
    - When `r` is `nil`, it reverts to the built‑in default (a masking redactor).
  - `GetRedactor` returns the current redactor, never `nil`.

### Built‑in behavior

- **NoopRedactor**
  - Renders arguments via `fmt.Sprintf("%v", args)`.
  - No redaction is applied; use only in trusted environments.
- **MaskingRedactor**
  - If there are no args, returns `"[]"`.
  - Otherwise returns a summary like `"[REDACTED xN]"`, where `N` is the number of arguments.
- **FieldMaskingRedactor**
  - Iterates through `args`.
  - When an argument is `map[string]interface{}`:
    - Produces a copy with specified keys replaced by `"[REDACTED]"` (case‑insensitive match).
  - Other argument types are formatted with `fmt.Sprintf`.

The default global redactor is a `MaskingRedactor`.

### Helper constructor

```go
func NewFieldMaskingRedactor(keys []string) *FieldMaskingRedactor
```

**Example: configuring field‑level redaction**

```go
redactor := crank.NewFieldMaskingRedactor([]string{
    "password",
    "secret",
    "token",
})

crank.SetRedactor(redactor)
```

With this configuration, when a job fails and `LoggingMiddleware` runs, map arguments will be logged with sensitive keys masked.

---

## Circuit Breaker

The circuit breaker protects your system from repeatedly executing failing job classes by temporarily halting their processing.

### Types

```go
type CircuitBreaker struct {
    // internal state
}

type BreakerConfig struct {
    FailureThreshold int           // failures within Window to open
    Window           time.Duration // sliding window for counting failures
    ResetTimeout     time.Duration // how long to stay Open before HalfOpen
}
```

### Constructor

```go
func NewCircuitBreaker(cfg BreakerConfig) *CircuitBreaker
```

- **Defaults**
  - `FailureThreshold`: default `5` when `<= 0`.
  - `Window`: default `1 * time.Minute` when `<= 0`.
  - `ResetTimeout`: default `60 * time.Second` when `<= 0`.

### Methods

```go
func (b *CircuitBreaker) Allow(class string) bool
func (b *CircuitBreaker) RecordSuccess(class string)
func (b *CircuitBreaker) RecordFailure(class string)
func (b *CircuitBreaker) IsOpen(class string) bool
```

- **Allow**
  - Returns `false` when the circuit is open for the class and still in the cool‑down window.
  - In half‑open state, returns `true` once (probe) and then `false` until a success or timeout.
- **RecordSuccess**
  - On success in half‑open: moves back to closed, clears failures.
  - On success in closed: clears recorded failures.
- **RecordFailure**
  - In half‑open: moves to open and sets `openUntil` based on `ResetTimeout`.
  - In closed:
    - Trims old failures outside the sliding window.
    - Appends the new failure time.
    - Opens the circuit when failures reach `FailureThreshold`.
- **IsOpen**
  - Returns `true` when the circuit for the given class is currently open and the cool‑down has not yet expired.

### Integration

- The default engine wiring does the following:
  - Wraps job handlers with `BreakerMiddleware(breaker)`.
  - Uses `breaker.Allow(job.Class)` in the fetcher to decide whether to process or temporarily requeue a job.

---

## Metrics & Job Statistics

### Job events and metrics handler

```go
type EventType int

type JobEvent struct {
    Type     EventType
    Job      *Job
    Queue    string
    Duration time.Duration
    Err      error
}

type MetricsHandler interface {
    HandleJobEvent(ctx context.Context, event JobEvent)
}
```

The processor can emit events for:

- `EventJobStarted`
- `EventJobSucceeded`
- `EventJobFailed`
- `EventJobRetryScheduled`
- `EventJobMovedToDead`

The engine’s processor emits these events internally. The `MetricsHandler` interface and event types are public for use in custom middleware or future engine APIs (e.g. `Engine.SetMetricsHandler`). For aggregate counts, use `engine.Stats()`.

### Queue statistics

```go
func (e *Engine) Stats() (*Stats, error)
```

- **Behavior**
  - Returns aggregate statistics from the broker: processed count, retry count, dead count, and per-queue sizes. Use this instead of accessing a broker directly.
- **Error behavior**
  - Returns an error when the broker fails to return stats.

**Example metrics handler**

```go
type PrometheusMetrics struct {
    // your counters / histograms here
}

func (p *PrometheusMetrics) HandleJobEvent(ctx context.Context, event crank.JobEvent) {
    switch event.Type {
    case crank.EventJobStarted:
        // increment in-flight counter
    case crank.EventJobSucceeded:
        // record latency and success count
    case crank.EventJobFailed:
        // record failure count, label by error type if desired
    case crank.EventJobRetryScheduled:
        // track retries
    case crank.EventJobMovedToDead:
        // track dead-letter promotions
    }
}
```

---

## Advanced Error‑Handling Patterns

Across validation, redaction, breaker, and metrics, the SDK follows these patterns:

- **Errors are returned, not panicked**
  - Validators, redactors, and broker operations return errors; the engine converts them into job‑level failures with retries and dead‑lettering as appropriate.
- **Panics are localized**
  - Job execution panics are caught by `RecoveryMiddleware` and turned into errors.
  - Metrics handler panics are caught and logged without stopping the engine.
- **Exponential backoff and dead‑lettering**
  - All execution‑time errors (including validation failures) pass through the same retry pipeline.
  - After exhausting retries, jobs move to a dead queue for later inspection.
- **Circuit breaker containment**
  - When a particular job class fails repeatedly within a short window, its circuit opens and the fetcher temporarily stops processing jobs of that class.
  - This prevents widespread degradation while still allowing other classes to proceed.

By composing these features—validators, redactors, circuit breaker, and metrics—you can build robust, observable job processing pipelines that fail fast, protect downstream systems, and surface rich telemetry for monitoring.

