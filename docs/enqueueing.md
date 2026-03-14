# Enqueueing Jobs

This document covers the client-side API for enqueueing jobs: the `Client` type, `Enqueue` / `EnqueueWithOptions`, global helpers, and error handling. All types and functions are in `github.com/ogwurujohnson/crank`.

---

## Getting a Client

You do **not** construct a broker or client in isolation. You obtain a **client** from:

- **`crank.New(brokerURL, opts...)`** – returns `(engine, client, err)`. Use `client` to enqueue.
- **`crank.QuickStart(configPath)`** – returns `(engine, client, err)` and also calls `SetGlobalClient(client)`.

There is no public broker type; the broker is chosen by URL (e.g. `redis://...`) or by config (`broker: redis`). The **Client** is the only type you use to push jobs.

---

## Core Types

### `type Client`

```go
type Client struct {
    // opaque; holds the broker internally
}
```

Obtained only as the second return value of `New` or `QuickStart`.

### `type Job`

```go
type Job struct {
    JID        string
    Class      string
    Args       []interface{}
    Queue      string
    Retry      int
    RetryCount int
    CreatedAt  float64
    EnqueuedAt float64
    Backtrace  bool
    State      JobState
    Metadata   map[string]interface{}
}
```

Jobs are created internally when you call `Enqueue` or `EnqueueWithOptions`. You can use `*Job` in validators and middleware. Defaults at creation: `Retry: 5`, `RetryCount: 0`, `Backtrace: false`, `State: JobStatePending`, `Metadata: map[]`.

### `type JobOptions`

```go
type JobOptions struct {
    Retry     *int
    Backtrace *bool
}
```

- **Retry**: max attempts (including first run). `nil` → use default (5).
- **Backtrace**: whether to capture extended backtrace. `nil` → false.

---

## Client methods

### `func (c *Client) Enqueue`

```go
func (c *Client) Enqueue(workerClass string, queue string, args ...interface{}) (string, error)
```

- **workerClass**: Must match a registered worker name (e.g. `"EmailWorker"`).
- **queue**: Queue name (e.g. `"default"`, `"critical"`). Must match a queue known to the engine (from options or config).
- **args**: Variadic arguments passed to `worker.Perform(ctx, args...)`.
- **Returns**: Job ID (`jid`) and an error. Errors are typically broker/connectivity failures.

**Example**

```go
engine, client, _ := crank.New("redis://localhost:6379/0", crank.WithQueues(crank.QueueOption{Name: "default", Weight: 1}))
jid, err := client.Enqueue("EmailWorker", "default", "user-123")
if err != nil {
    log.Printf("enqueue failed: %v", err)
}
```

### `func (c *Client) EnqueueWithOptions`

```go
func (c *Client) EnqueueWithOptions(
    workerClass string,
    queue string,
    options *JobOptions,
    args ...interface{},
) (string, error)
```

Same as `Enqueue`, but applies `options` when non-nil: `Retry` and/or `Backtrace` override job defaults.

**Example**

```go
retries := 10
opts := &crank.JobOptions{Retry: &retries}
jid, err := client.EnqueueWithOptions("ReportWorker", "reports", opts, 2024, "q1")
```

---

## Global enqueue helpers

After calling `SetGlobalClient(client)`, you can enqueue from anywhere without passing the client.

### `func SetGlobalClient`

```go
func SetGlobalClient(c *Client)
```

Sets the client used by the global `Enqueue` and `EnqueueWithOptions` functions. `QuickStart` does this for you.

### `func GetGlobalClient`

```go
func GetGlobalClient() *Client
```

Returns the current global client, or `nil` if never set.

### `func Enqueue` (global)

```go
func Enqueue(workerClass string, queue string, args ...interface{}) (string, error)
```

Equivalent to `GetGlobalClient().Enqueue(...)`. Returns an error if the global client is nil: `global client not initialized. Call SetGlobalClient first`.

### `func EnqueueWithOptions` (global)

```go
func EnqueueWithOptions(
    workerClass string,
    queue string,
    options *JobOptions,
    args ...interface{},
) (string, error)
```

Same as above for the options variant.

**Example**

```go
// Bootstrap (e.g. in main)
engine, client, _ := crank.New("redis://localhost:6379/0", ...)
crank.SetGlobalClient(client)
engine.Start()

// Elsewhere in the app
jid, err := crank.Enqueue("EmailWorker", "default", userID)
```

---

## JSON helpers

```go
func (j *Job) ToJSON() ([]byte, error)
func FromJSON(data []byte) (*Job, error)
```

Use when you need to serialize or deserialize jobs (e.g. in middleware or tests). `FromJSON` defaults `State` to `JobStatePending` when missing.

---

## Error handling

- **Broker/connectivity**: Enqueue can fail if the broker is unreachable or the URL is invalid. Errors are returned (e.g. `failed to enqueue job: ...`). Create the engine/client at startup and handle errors there; retry or back off as needed.
- **Global client**: Calling `Enqueue` or `EnqueueWithOptions` without having called `SetGlobalClient` returns `global client not initialized. Call SetGlobalClient first`.
- **Validation**: Job validation runs at **execution** time, not at enqueue time. Invalid jobs will fail in the worker and go through the normal retry/dead path.
