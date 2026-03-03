## Getting started with Crank

Crank is a Sidekiq-inspired, high-performance background job processor for Go. It uses a pluggable broker (Redis by default), a managed worker pool, and channel-based backpressure so jobs are never pulled faster than workers can process them.

### Installation and requirements

- **Go**: 1.21+
- **Redis**: 7.0+ (or compatible implementation such as Valkey or Dragonfly)

Install the SDK:

```bash
go get github.com/ogwurujohnson/crank
```

### Core concepts

- **Broker**: Backend that stores jobs and stats (Redis by default), behind a `Broker` interface.
- **Client**: Enqueues jobs into queues via the broker.
- **Engine / Processor**: Dequeues jobs from queues and dispatches them to workers using a worker pool with backpressure.
- **Circuit breaker**: Tracks per-job-class failures and temporarily pauses classes that are repeatedly failing, to avoid hammering unhealthy work.
- **Worker**: Your implementation that performs the actual job.
- **Queue**: Named stream of jobs with a weight that controls polling priority.

### Minimal producer example

Define a worker and enqueue a job from your application:

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ogwurujohnson/crank"
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

### Running workers with the standalone binary

The repository includes a `cmd/crank` worker binary. It loads configuration and starts the engine for you.

```bash
# From the repo root, with Redis running:
go run ./cmd/crank/ -C config/crank.yml
```

Or build and run:

```bash
make build
./bin/crank -C config/crank.yml
```

### Basic configuration

Crank does not read any configuration automatically. You are responsible for constructing a `*crank.Config` either from YAML or in code.

#### YAML configuration via LoadConfig

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

By default, the engine wires in a per-class circuit breaker. You do not need to configure it to benefit from it: if a particular job class keeps failing within a short window, Crank will temporarily stop fetching jobs of that class, wait for a cool-down period, and then slowly resume processing with a single probe job.

Example YAML:

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

Key fields:

- **concurrency**: Number of worker goroutines (and effective backpressure limit).
- **queues**: Queue name and weight; higher weight means more polling share.
- **timeout**: Job execution timeout in seconds.
- **redis.url**: Redis connection URL (can be overridden by `REDIS_URL`).

#### Configuration in code

```go
config := &crank.Config{
	Concurrency: 10,
	Timeout:     8,
	Queues:      []crank.QueueConfig{{Name: "default", Weight: 1}},
	Redis: crank.RedisConfig{
		URL:            os.Getenv("REDIS_URL"),
		NetworkTimeout: 5,
	},
	Logger: crank.NopLogger(), // or your own adapter
}
engine, err := crank.NewEngine(config, broker)
if err != nil {
	log.Fatal(err)
}
```

### Running the engine inside your app

Instead of using the standalone binary, you can embed the engine in your own process:

```go
config, _ := crank.LoadConfig("config/crank.yml")
broker, _ := crank.NewRedisClient(config.Redis.URL, config.Redis.GetNetworkTimeout())
defer broker.Close()

engine, _ := crank.NewEngine(config, broker)
engine.RegisterMany(map[string]crank.Worker{
	"EmailWorker": &EmailWorker{},
})
if err := engine.Start(); err != nil {
	panic(err)
}
defer engine.Stop()

// Your HTTP server or other long-running work here
```

### Next steps

- For a deeper look at how Crank works internally (worker pool, backpressure, broker interface, middleware), see [Architecture and internals](architecture-and-internals.md).
- For tuning, custom logging, TLS, metrics, and more advanced options, see [Advanced configuration](advanced-configuration.md).

