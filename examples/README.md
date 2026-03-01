# Crank Examples

## Simple Worker Example

The `simple_worker/` example demonstrates:
- Registering workers
- Enqueueing jobs
- Using job options

Run it:
```bash
go run ./examples/simple_worker/
```

Then start the worker process in another terminal:
```bash
go run ./cmd/crank/ -C config/crank.yml
```

## Web Server Example

The `web_server/` example demonstrates:
- Running Crank Web UI
- Enqueueing jobs via HTTP API
- Monitoring jobs through the web interface

Run it:
```bash
go run ./examples/web_server/
```

Then:
1. Visit http://localhost:8080/crank to see the Web UI
2. Enqueue a job: `curl -X POST "http://localhost:8080/api/jobs?user_id=123"`

## Creating Your Own Workers

```go
package main

import (
    "context"
    "github.com/quest/crank"
)

type MyWorker struct{}

func (w *MyWorker) Perform(ctx context.Context, args ...interface{}) error {
    // Your job logic here
    // args[0], args[1], etc. are your job arguments
    return nil
}

func main() {
    // Register the worker
    crank.RegisterWorker("MyWorker", &MyWorker{})
    
    // Enqueue a job
    crank.Enqueue("MyWorker", "default", arg1, arg2, arg3)
}
```

## Worker Best Practices

1. **Idempotency**: Make workers idempotent - running the same job multiple times should be safe
2. **Error Handling**: Return errors for failures, Crank will retry automatically
3. **Context**: Use the context for cancellation and timeouts
4. **Resource Management**: Clean up resources (DB connections, file handles, etc.)
5. **Logging**: Log important events for debugging
