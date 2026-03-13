## Configuration & Redis Broker

This document describes how to configure Crank via YAML and environment variables, and how to set up the Redis‑backed broker used by the client and engine.

All symbols are exposed via `github.com/ogwurujohnson/crank`.

---

## Configuration Types

### `type Config`

```go
type Config struct {
    Concurrency int           `yaml:"concurrency"`
    Queues      []QueueConfig `yaml:"queues"`
    Timeout     int           `yaml:"timeout"`
    Verbose     bool          `yaml:"verbose"`
    Redis       RedisConfig   `yaml:"redis"`
    Logger      Logger        `yaml:"-"`
}
```

- **Fields**
  - `Concurrency` (**optional**, default: `10`):
    - Number of concurrent worker goroutines.
  - `Queues` (**optional**, default: derived; uses `["default"]` when empty):
    - List of queues and weights (see `QueueConfig`).
  - `Timeout` (**optional**, default: `8` seconds):
    - Per‑job execution timeout in seconds.
  - `Verbose` (**optional**):
    - Reserved flag for more detailed logging; behavior depends on your logger implementation.
  - `Redis` (**required in practice, but defaults applied when missing**):
    - Redis connection details (see `RedisConfig`).
  - `Logger` (**optional**):
    - Your implementation of `Logger`. When not provided, a no‑op logger is used.

### `type QueueConfig`

```go
// QueueConfig: YAML accepts [name, weight] or {name, weight, priority}. Priority reserved.
type QueueConfig struct {
    Name     string `yaml:"name"`
    Weight   int    `yaml:"weight"`
    Priority int    `yaml:"priority"`
}
```

- **YAML forms**
  - Short form (array):

```yaml
queues:
  - [default, 5]
  - [critical, 10]
```

  - Long form (map):

```yaml
queues:
  - name: default
    weight: 5
    priority: 0   # reserved for future use
```

- **Defaults**
  - `Weight`: defaults to `1` when omitted or zero.

### `type RedisConfig`

```go
type RedisConfig struct {
    URL                   string `yaml:"url"`
    NetworkTimeout        int    `yaml:"network_timeout"`
    UseTLS                bool   `yaml:"use_tls"`
    TLSInsecureSkipVerify bool   `yaml:"tls_insecure_skip_verify"`
}
```

- **Fields**
  - `URL` (**optional**, but effectively required to connect to Redis):
    - Redis connection URL, e.g. `redis://localhost:6379/0`.
    - If empty, `LoadConfig` will:
      - Read from environment variable `REDIS_URL`, or
      - Fallback to `redis://localhost:6379/0`.
  - `NetworkTimeout` (**optional**, default: `5` seconds):
    - Network timeout (seconds) for Redis operations.
  - `UseTLS` (**optional**, default: `false`):
    - Whether to use TLS for the Redis connection.
  - `TLSInsecureSkipVerify` (**optional**, default: `false`):
    - Whether to skip TLS certificate verification (use with care, typically only in development).

---

## Loading Configuration

### `func LoadConfig`

```go
func LoadConfig(path string) (*Config, error)
```

- **Parameters**
  - `path` (**required**): filesystem path to a YAML configuration file.
- **Behavior**
  - Reads the file and unmarshals it into `Config`.
  - Applies defaults:
    - `Concurrency` defaults to `10` if `0`.
    - `Timeout` defaults to `8` seconds if `0`.
    - `Redis.URL`:
      - If empty, set from `REDIS_URL` environment variable if present.
      - Otherwise defaults to `redis://localhost:6379/0`.
    - `Redis.NetworkTimeout` defaults to `5` seconds if `0`.
- **Error behavior**
  - Returns wrapped errors when:
    - The file cannot be read:

```go
fmt.Errorf("failed to read config file: %w", err)
```

    - The YAML cannot be parsed:

```go
fmt.Errorf("failed to parse config: %w", err)
```

### Helpers

```go
func (c *Config) GetTimeout() time.Duration
func (c *RedisConfig) GetNetworkTimeout() time.Duration
```

- `GetTimeout`:
  - Returns `time.Duration(c.Timeout) * time.Second`.
- `GetNetworkTimeout`:
  - Returns `time.Duration(c.NetworkTimeout) * time.Second`.

---

## Example YAML Configuration

```yaml
concurrency: 20
timeout: 15
verbose: true

queues:
  - [default, 5]
  - [critical, 10]

redis:
  url: redis://localhost:6379/0
  network_timeout: 5
  use_tls: false
  tls_insecure_skip_verify: false
```

**Environment integration**

- If `redis.url` is omitted, `REDIS_URL` is used if set:

```bash
export REDIS_URL=redis://my-redis-host:6379/0
```

---

## Redis Broker

Crank uses a broker abstraction to interact with Redis. The SDK exports a Redis‑backed implementation and helper constructors.

### Types

```go
type Broker interface {
    // internal interface used by engine and client
}

type RedisClient = RedisBroker

type RedisBrokerConfig struct {
    URL                   string
    Timeout               time.Duration
    UseTLS                bool
    TLSInsecureSkipVerify bool
}
```

> Note: `Broker` is an internal interface; external users normally work with `RedisClient` / `RedisBrokerConfig`.

### Constructors

```go
func NewRedisClient(url string, timeout time.Duration) (*RedisClient, error)

func NewRedisClientWithConfig(cfg RedisBrokerConfig) (*RedisClient, error)
```

- **`NewRedisClient`**
  - **Parameters**
    - `url` (**required**): Redis URL (e.g. `redis://localhost:6379/0`).
    - `timeout` (**required**): network timeout used for Redis operations.
  - **Behavior**
    - Creates a Redis broker with sane defaults; TLS is disabled unless configured via the URL.
  - **Error behavior**
    - Returns errors if the URL is invalid or Redis cannot be reached during initialization, wrapped by the underlying broker implementation.

- **`NewRedisClientWithConfig`**
  - **Parameters**
    - `cfg RedisBrokerConfig` (**required**): full configuration for the Redis broker.
      - `URL` (**required**).
      - `Timeout` (**required**).
      - `UseTLS` (**optional**).
      - `TLSInsecureSkipVerify` (**optional**).
  - **Behavior**
    - Same as `NewRedisClient`, but gives explicit control over TLS options.

---

## Putting It Together

**Bootstrap example**

```go
cfg, err := crank.LoadConfig("config/crank.yml")
if err != nil {
    log.Fatalf("failed to load config: %v", err)
}

redis, err := crank.NewRedisClientWithConfig(crank.RedisBrokerConfig{
    URL:                   cfg.Redis.URL,
    Timeout:               cfg.Redis.GetNetworkTimeout(),
    UseTLS:                cfg.Redis.UseTLS,
    TLSInsecureSkipVerify: cfg.Redis.TLSInsecureSkipVerify,
})
if err != nil {
    log.Fatalf("failed to create redis client: %v", err)
}

engine, err := crank.NewEngine(cfg, redis)
if err != nil {
    log.Fatalf("failed to create engine: %v", err)
}

client := crank.NewClient(redis)
crank.SetGlobalClient(client)
```

---

## Configuration‑Related Error Handling

When working with configuration and the Redis broker, typical error cases include:

- **Missing or unreadable config file**
  - `LoadConfig` returns `failed to read config file: ...`.
  - Recommended to treat as fatal at startup.
- **Invalid YAML**
  - `LoadConfig` returns `failed to parse config: ...`.
  - Recommended to treat as fatal at startup.
- **Invalid Redis URL or connection issues**
  - `NewRedisClient` / `NewRedisClientWithConfig` return an error from the underlying Redis client.
  - Recommended to log and exit at startup; in long‑running apps you may implement retries around initialization.

By handling these errors upfront, you ensure the engine and client are only created when the system is in a valid, connected state.

