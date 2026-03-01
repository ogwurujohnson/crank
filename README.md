# Sidekiq-Go

A high-performance background job processor for Go, inspired by [Sidekiq](https://github.com/sidekiq/sidekiq). Uses goroutines for concurrent job processing and Redis as the job store.

## Features

- **Thread-safe job processing** using goroutines
- **Redis-backed job queue** with priority support
- **Automatic retries** with exponential backoff
- **Web UI** for monitoring and job management
- **Weighted queues** for priority-based processing
- **Middleware support** for custom processing hooks
- **YAML configuration** support
- **Production-ready** with proper error handling and logging
- **Graceful shutdown** support
- **Queue statistics** and monitoring

## Requirements

- Go 1.21+
- Redis 7.0+ (or compatible: Valkey, Dragonfly)

## Installation

```bash
go mod download
```

Or add to your project:
```bash
go get github.com/quest/sidekiq-go
```

## Quick Start

### 1. Define a Worker

```go
package main

import (
    "context"
    "fmt"
    "time"
    "github.com/quest/sidekiq-go"
)

type EmailWorker struct{}

func (w *EmailWorker) Perform(ctx context.Context, args ...interface{}) error {
    userID := args[0].(float64) // JSON numbers come as float64
    fmt.Printf("Sending email to user %v\n", userID)
    // Your email sending logic here
    return nil
}

func main() {
    // Connect to Redis
    redis, _ := sidekiq.NewRedisClient("redis://localhost:6379/0", 5*time.Second)
    defer redis.Close()
    
    // Initialize client
    client := sidekiq.NewClient(redis)
    sidekiq.SetGlobalClient(client)
    
    // Register worker
    sidekiq.RegisterWorker("EmailWorker", &EmailWorker{})
    
    // Enqueue a job
    jid, _ := sidekiq.Enqueue("EmailWorker", "default", 123)
    fmt.Printf("Enqueued job: %s\n", jid)
}
```

### 2. Start the Worker Process

```bash
# Using make
make run

# Or directly
go run cmd/sidekiq/main.go -C config/sidekiq.yml

# Or build first
make build
./bin/sidekiq -C config/sidekiq.yml
```

### 3. Configuration

Create `config/sidekiq.yml`:

```yaml
concurrency: 10
queues:
  - [critical, 5]  # Queue name and weight (priority)
  - [default, 3]
  - [low, 1]
timeout: 8
verbose: true
redis:
  url: redis://localhost:6379/0
  network_timeout: 5
```

Configuration options:
- `concurrency`: Number of concurrent workers (goroutines)
- `queues`: List of queues with weights (higher weight = more priority)
- `timeout`: Job timeout in seconds
- `verbose`: Enable verbose logging
- `redis.url`: Redis connection URL (can use `REDIS_URL` env var)
- `redis.network_timeout`: Network timeout in seconds

## Web UI

Mount the Web UI in your application:

```go
import (
    "github.com/gorilla/mux"
    "github.com/quest/sidekiq-go/web"
)

// In your HTTP server setup
router := mux.NewRouter()
web.Mount(router, "/sidekiq", redisClient)
```

Access at `http://localhost:8080/sidekiq` to view:
- Queue statistics
- Queue sizes
- Retry and dead job counts
- Clear queues

## Advanced Usage

### Job Options

```go
options := &sidekiq.JobOptions{
    Retry:     intPtr(3),
    Backtrace: boolPtr(true),
}
jid, _ := sidekiq.EnqueueWithOptions("EmailWorker", "critical", options, 123)
```

### Middleware

```go
sidekiq.AddMiddleware(func(ctx context.Context, job *sidekiq.Job, next func() error) error {
    start := time.Now()
    defer func() {
        log.Printf("Job %s took %v", job.JID, time.Since(start))
    }()
    return next()
})
```

### Queue Management

```go
queue := sidekiq.NewQueue("default", redisClient)
size, _ := queue.Size()
queue.Clear() // Clear all jobs

stats, _ := sidekiq.GetStats(redisClient)
fmt.Printf("Processed: %d, Retry: %d, Dead: %d\n", 
    stats.Processed, stats.Retry, stats.Dead)
```

## Architecture

- **Client**: Enqueues jobs to Redis
- **Server/Processor**: Processes jobs using worker pool with goroutines
- **Worker**: Implements the `Worker` interface (`Perform` method)
- **Queue**: Redis lists for job storage
- **Retry**: Automatic retry with exponential backoff (stored in Redis sorted set)
- **Dead Jobs**: Jobs that exceeded max retries (stored in Redis sorted set)

## Examples

See the `examples/` directory for:
- Simple worker example
- Web server with Sidekiq UI
- Best practices

Run examples:
```bash
make examples
```

## Deployment

See [DEPLOYMENT.md](DEPLOYMENT.md) for:
- Systemd service setup
- Docker deployment
- Kubernetes configuration
- Production best practices

## Performance

Sidekiq-Go is designed for high throughput:
- Uses goroutines for efficient concurrency
- Minimal overhead with direct Redis operations
- Efficient job serialization/deserialization
- Weighted queue polling for priority processing

## Comparison with Sidekiq (Ruby)

| Feature | Sidekiq (Ruby) | Sidekiq-Go |
|---------|---------------|------------|
| Language | Ruby | Go |
| Concurrency Model | Threads | Goroutines |
| Performance | High | Very High |
| Memory Usage | Moderate | Low |
| Type Safety | Dynamic | Static |
| Deployment | Ruby-specific | Standard Go binaries |

## Contributing

Contributions are welcome! Please:
1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## License

MIT

## References

- [Original Sidekiq (Ruby)](https://github.com/sidekiq/sidekiq)
- [Sidekiq Documentation](https://github.com/sidekiq/sidekiq/wiki)
