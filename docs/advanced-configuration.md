## Advanced configuration

This page covers configuration options, logging, middleware, metrics, TLS, queue tuning, and other advanced features available in the Crank SDK.

### Configuration overview

Crank uses a `Config` struct to control concurrency, queues, timeouts, Redis, logging, and more. You can either:

- Load it from YAML using `crank.LoadConfig(path)`, or
- Construct it directly in Go and pass it to `NewEngine`.

Example (YAML + loader):

```go
config, err := crank.LoadConfig("config/crank.yml")
if err != nil {
	log.Fatal(err)
}
broker, err := crank.NewRedisClient(config.Redis.URL, config.Redis.GetNetworkTimeout())
if err != nil {
	log.Fatal(err)
}
defer broker.Close()

engine, err := crank.NewEngine(config, broker)
if err != nil {
	log.Fatal(err)
}
```

### Concurrency, queues, and timeouts

Key `Config` fields:

- **Concurrency**: Number of worker goroutines; governs parallelism and backpressure.
- **Queues**: Slice of queue configs with `Name` and `Weight`.
- **Timeout**: Job execution timeout (seconds).

YAML example:

```yaml
concurrency: 20
queues:
  - [critical, 5]
  - [default, 3]
  - [low, 1]
timeout: 30
redis:
  url: redis://localhost:6379/0
  network_timeout: 5
```

Guidance:

- Start with concurrency equal to the number of CPU cores, then tune based on workload and I/O.
- Use higher weights for latency-sensitive queues (e.g. `critical`) and lower weights for background work.
- Set `timeout` just above the 99th percentile duration of your jobs.

### Redis and TLS options

The `RedisConfig` section controls how Crank connects to Redis:

- **url**: Redis URL (`redis://` or `rediss://`).
- **network_timeout**: Timeout for network operations (seconds).
- **use_tls**: Enable TLS when using `redis://` URLs.
- **tls_insecure_skip_verify**: Disable certificate verification (for development only).

Example:

```yaml
redis:
  url: rediss://redis.example.com:6379/0
  network_timeout: 3
  use_tls: true
  tls_insecure_skip_verify: false
```

You can override the URL via `REDIS_URL` and keep configuration files environment-agnostic.

### Logging and structured logging

The engine logs lifecycle and job-level events via the `Logger` interface:

- `Debug(msg string, args ...any)`
- `Info(msg string, args ...any)`
- `Warn(msg string, args ...any)`
- `Error(msg string, args ...any)`

If `Config.Logger` is `nil`, a no-op logger is used.

Example: integrate with `log/slog` using a small adapter:

```go
import "log/slog"

type slogAdapter struct{ *slog.Logger }

func (a slogAdapter) Debug(msg string, args ...any) { a.Logger.Debug(msg, args...) }
func (a slogAdapter) Info(msg string, args ...any)  { a.Logger.Info(msg, args...) }
func (a slogAdapter) Warn(msg string, args ...any)  { a.Logger.Warn(msg, args...) }
func (a slogAdapter) Error(msg string, args ...any) { a.Logger.Error(msg, args...) }

config := &crank.Config{
	// ...
	Logger: slogAdapter{Logger: slog.Default()},
}
engine, _ := crank.NewEngine(config, broker)
```

To disable logging explicitly:

```go
config.Logger = crank.NopLogger()
```

### Middleware configuration

Middleware allows you to add cross-cutting concerns around job execution (logging, tracing, rate limiting, metrics, etc.).

Attach middleware to the engine:

```go
engine, _ := crank.NewEngine(config, broker)

engine.Use(crank.LoggingMiddleware(config.Logger))
engine.Use(crank.RecoveryMiddleware(config.Logger))

engine.Use(func(next crank.Handler) crank.Handler {
	return func(ctx context.Context, job *crank.Job) error {
		// Custom behavior before
		err := next(ctx, job)
		// Custom behavior after
		return err
	}
})
```

Crank composes the middleware chain once at startup, yielding a pre-compiled handler without per-job overhead.

### Payload validation and redaction

Use validators to harden the system against malformed or unexpected jobs:

- `SafeClassPattern()`: Restrict worker class names to a safe pattern.
- `MaxArgsCount(n)`: Limit argument count per job.
- `ChainValidator{...}`: Compose multiple validators.

Example:

```go
crank.SetValidator(crank.ChainValidator{
	crank.SafeClassPattern(),
	crank.MaxArgsCount(10),
})
```

Use redactors to protect sensitive information from logs:

```go
crank.SetRedactor(crank.NewFieldMaskingRedactor([]string{
	"password",
	"token",
	"authorization",
}))
```

The logging middleware uses your redactor before emitting job details.

### Metrics and Prometheus integration

Crank exposes a non-blocking internal event bus via `SetMetricsHandler`. Workers emit `JobEvent`s into a buffered channel; a background goroutine invokes your handler. If your metrics backend is slow, events are dropped rather than blocking job execution.

Example with Prometheus:

```go
var (
	jobDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "crank_job_duration_seconds",
			Help: "Time spent executing jobs.",
		},
		[]string{"class", "queue", "state"},
	)
	jobTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "crank_jobs_total",
			Help: "Total number of processed jobs.",
		},
		[]string{"class", "queue", "state"},
	)
)

func init() {
	prometheus.MustRegister(jobDuration, jobTotal)
}

type promMetrics struct{}

func (promMetrics) HandleJobEvent(ctx context.Context, e crank.JobEvent) {
	class := e.Job.Class
	queue := e.Queue

	switch e.Type {
	case crank.EventJobSucceeded:
		jobDuration.WithLabelValues(class, queue, "success").Observe(e.Duration.Seconds())
		jobTotal.WithLabelValues(class, queue, "success").Inc()
	case crank.EventJobFailed:
		jobDuration.WithLabelValues(class, queue, "failed").Observe(e.Duration.Seconds())
		jobTotal.WithLabelValues(class, queue, "failed").Inc()
	case crank.EventJobRetryScheduled:
		jobTotal.WithLabelValues(class, queue, "retry_scheduled").Inc()
	case crank.EventJobMovedToDead:
		jobTotal.WithLabelValues(class, queue, "dead").Inc()
	}
}

func main() {
	config, _ := crank.LoadConfig("config/crank.yml")
	broker, _ := crank.NewRedisClient(config.Redis.URL, config.Redis.GetNetworkTimeout())
	defer broker.Close()

	engine, _ := crank.NewEngine(config, broker)
	engine.SetMetricsHandler(promMetrics{})

	http.Handle("/metrics", promhttp.Handler())
	go http.ListenAndServe(":2112", nil)

	if err := engine.Start(); err != nil {
		panic(err)
	}
	defer engine.Stop()

	select {}
}
```

### Queue and stats APIs

You can inspect queues and global stats programmatically:

```go
queue := crank.NewQueue("default", broker)
size, _ := queue.Size()
_ = queue.Clear()

stats, _ := crank.GetStats(broker)
fmt.Printf("Processed: %d, Retry: %d, Dead: %d\n", stats.Processed, stats.Retry, stats.Dead)
// stats.Queues is map[string]int64
```

`GetStats` parses broker responses defensively and returns an error instead of panicking on invalid data.

### Operations

- Use the `web` UI (mounted via `web.Mount`) to inspect queues, retries, and dead jobs.
- Protect destructive endpoints (like clear-queue) with authentication or network controls.

### Custom brokers and extensions

For fully customized setups:

- Implement the `Broker` interface to back Crank with a different storage system.
- Add middleware for tracing, distributed logging, or rate limiting.
- Use the metrics handler to integrate with non-Prometheus observability stacks.

For a conceptual overview of how these pieces fit together, see [Architecture and internals](architecture-and-internals.md). For a basic introduction and examples, see [Getting started](getting-started.md).

