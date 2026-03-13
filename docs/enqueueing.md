## Enqueueing Jobs

This document covers the client‑side API for enqueueing jobs into Crank, including method signatures, parameters, and error patterns.

All types and functions referenced here are available through the top‑level `github.com/ogwurujohnson/crank` package (they are re‑exports of the internal SDK and payload packages).

---

## Core Types

### `type Client`

```go
type Client struct {
    // wraps an internal broker.Broker
}
```

Created via:

```go
func NewClient(b Broker) *Client
```

- **Parameters**
  - `b Broker` (**required**): a broker implementation, typically a Redis broker created with `NewRedisClient` or `NewRedisClientWithConfig`.
- **Behavior**
  - `Client` is a thin wrapper around the broker that constructs `Job` payloads with sensible defaults and pushes them to the configured Redis queues.

### `type Job`

```go
type Job struct {
    JID        string                 `json:"jid"`
    Class      string                 `json:"class"`
    Args       []interface{}          `json:"args"`
    Queue      string                 `json:"queue"`
    Retry      int                    `json:"retry"`
    RetryCount int                    `json:"retry_count"`
    CreatedAt  float64                `json:"created_at"`
    EnqueuedAt float64                `json:"enqueued_at"`
    Backtrace  bool                   `json:"backtrace"`
    State      JobState               `json:"state,omitempty"`
    Metadata   map[string]interface{} `json:"metadata,omitempty"`
}
```

Created via:

```go
func NewJob(workerClass string, queue string, args ...interface{}) *Job
```

- **Defaults**
  - `Retry`: **5** attempts.
  - `RetryCount`: `0` at creation.
  - `Backtrace`: `false`.
  - `State`: `JobStatePending`.
  - `Metadata`: initialized to an empty map.

### `type JobOptions`

```go
type JobOptions struct {
    Retry     *int
    Backtrace *bool
}
```

- **Retry** (optional): maximum number of attempts, including the first run. When `nil`, the default (5) from `NewJob` is used.
- **Backtrace** (optional): whether to capture extended backtrace information. When `nil`, defaults to `false`.

---

## Client Methods

### `func (c *Client) Enqueue`

```go
func (c *Client) Enqueue(workerClass string, queue string, args ...interface{}) (string, error)
```

- **Parameters**
  - `workerClass` (**required**): name of the worker class to execute (e.g. `"EmailWorker"`). This must match the string used when registering the worker with the engine.
  - `queue` (**required**): name of the target queue (e.g. `"default"`, `"critical"`).
  - `args` (**optional**): variadic list of arguments passed to the worker’s `Perform` method.
- **Returns**
  - `string`: the generated job ID (`jid`).
  - `error`: non‑`nil` when the job could not be enqueued.
- **Error behavior**
  - Wraps broker errors as:
    - `failed to enqueue job: <underlying error>`
  - Does not validate `workerClass` or `args` at enqueue time beyond basic serialization; validation is performed when the job is processed.

**Example**

```go
client := crank.NewClient(redisBroker)
jid, err := client.Enqueue("EmailWorker", "default", "user-123")
if err != nil {
    log.Printf("failed to enqueue job: %v", err)
}
log.Printf("enqueued job %s", jid)
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

- **Parameters**
  - `workerClass` (**required**): worker class name.
  - `queue` (**required**): queue name.
  - `options` (**optional**):
    - When non‑`nil`, overrides default retry and backtrace behavior.
    - When `nil`, defaults from `NewJob` are used.
  - `args` (**optional**): arguments passed to the worker.
- **Returns**
  - `string`: job ID.
  - `error`: enqueuing error, if any.
- **Behavior**
  - Constructs a `Job` via `NewJob`.
  - Applies:
    - `SetRetry(*options.Retry)` when `options` and `options.Retry` are non‑`nil`.
    - `SetBacktrace(*options.Backtrace)` when `options` and `options.Backtrace` are non‑`nil`.
  - Enqueues the job via the underlying broker.

**Example**

```go
retries := 10
backtrace := true
opts := &crank.JobOptions{
    Retry:     &retries,
    Backtrace: &backtrace,
}

jid, err := client.EnqueueWithOptions(
    "ReportWorker",
    "reports",
    opts,
    2024, "quarter-1",
)
if err != nil {
    log.Printf("failed to enqueue report job: %v", err)
}
```

---

## Global Enqueue Helpers

Crank provides convenience functions that use a globally configured client. This can be useful in larger applications where jobs are enqueued from many different packages.

### `func SetGlobalClient`

```go
func SetGlobalClient(c *Client)
```

- **Parameters**
  - `c` (**required**): the client instance to be used by global helpers.
- **Behavior**
  - Stores `c` in a package‑level variable. Subsequent calls to `Enqueue` / `EnqueueWithOptions` (global variants) will use this client.

### `func GetGlobalClient`

```go
func GetGlobalClient() *Client
```

- **Returns**
  - The current global client, or `nil` if none has been configured.

### `func Enqueue`

```go
func Enqueue(workerClass string, queue string, args ...interface{}) (string, error)
```

- **Behavior**
  - Delegates to `globalClient.Enqueue`.
- **Error behavior**
  - Returns an error if the global client has not been initialized:
    - `global client not initialized. Call SetGlobalClient first`
  - Otherwise, returns the same errors as `Client.Enqueue`.

### `func EnqueueWithOptions`

```go
func EnqueueWithOptions(
    workerClass string,
    queue string,
    options *JobOptions,
    args ...interface{},
) (string, error)
```

- **Behavior**
  - Delegates to `globalClient.EnqueueWithOptions`.
- **Error behavior**
  - Returns an error if the global client has not been initialized:
    - `global client not initialized. Call SetGlobalClient first`
  - Otherwise, returns the same errors as `Client.EnqueueWithOptions`.

**Example: global client pattern**

```go
// Somewhere in your service bootstrap:
cfg, err := crank.LoadConfig("config/crank.yml")
if err != nil {
    log.Fatalf("failed to load config: %v", err)
}

redis, err := crank.NewRedisClient(cfg.Redis.URL, cfg.Redis.GetNetworkTimeout())
if err != nil {
    log.Fatalf("failed to create redis client: %v", err)
}

client := crank.NewClient(redis)
crank.SetGlobalClient(client)

// Later, in any package:
func EnqueueWelcomeEmail(userID string) error {
    _, err := crank.Enqueue("EmailWorker", "default", userID)
    return err
}
```

---

## JSON Helpers

Crank exposes helpers to convert jobs to and from JSON.

### `func (j *Job) ToJSON`

```go
func (j *Job) ToJSON() ([]byte, error)
```

- Serializes the job to JSON.
- Returns an error if serialization fails.

### `func FromJSON`

```go
func FromJSON(data []byte) (*Job, error)
```

- **Parameters**
  - `data` (**required**): JSON bytes representing a job.
- **Behavior**
  - Unmarshals JSON into a `Job`.
  - When `state` is omitted or empty, defaults it to `JobStatePending`.
- **Error behavior**
  - Returns an error wrapped as:
    - `failed to unmarshal job: <underlying error>`

---

## Enqueue‑Side Error Handling Patterns

When working with the enqueueing API, you should be prepared to handle:

- **Configuration errors**
  - Failures when constructing the broker (e.g. invalid Redis URL) will surface before you create the `Client`.
- **Broker connectivity errors**
  - Transient Redis issues during `Enqueue` or `EnqueueWithOptions` result in wrapped errors such as:
    - `failed to enqueue job: dial tcp ...`
- **Global client misuse**
  - Calling the global helpers before `SetGlobalClient` yields:
    - `global client not initialized. Call SetGlobalClient first`
- **Payload serialization issues**
  - Rare in practice (args are stored as generic interfaces), but will surface as standard JSON marshaling errors from `ToJSON` / `FromJSON`.

In all cases, the enqueueing API follows idiomatic Go practice: operations return an `error` and do not panic. Your application should log and/or retry based on its own policies.

