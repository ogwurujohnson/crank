# Configuration

This document describes how to configure Crank via YAML when using `QuickStart(configPath)`. All symbols referenced here are from `github.com/ogwurujohnson/crank`.

---

## How configuration is used

- **`New(brokerURL, opts...)`** – You do **not** use a config file. You pass a broker URL and optional functional options (`WithConcurrency`, `WithTimeout`, `WithQueues`, etc.). See `docs/engine.md`.
- **`QuickStart(configPath)`** – Loads a YAML file from `configPath`, validates it, creates the broker and engine from it. The file must define `broker` and the matching backend section (`redis` or `nats`).

There is no public `LoadConfig`; config loading is internal to `QuickStart`.

---

## YAML structure

Top-level fields:

| Field | Type | Description |
|-------|------|-------------|
| `broker` | string | Backend to use: `redis`, `nats`, or `rabbitmq`. Default: `redis` if omitted. |
| `broker_url` | string | Optional fallback URL when the backend’s `url` (e.g. `redis.url`) is empty. |
| `concurrency` | int | Number of concurrent workers. Default: `10`. |
| `timeout` | int | Per-job execution timeout in **seconds**. Default: `8`. |
| `queues` | list | Queue names and weights (see below). Default: one queue `default` with weight 1. |
| `verbose` | bool | Reserved for future use. |
| `redis` | object | **Required when `broker: redis`.** URL and connection options. |
| `nats` | object | **Required when `broker: nats`.** URL and timeout (NATS not yet implemented). |

---

## Broker selection

Set `broker` to choose the backend. The **matching section** must be present and provide a URL.

- **`broker: redis`** (default)
  - The `redis` section must be present. Use `redis.url` (or `broker_url` or env `REDIS_URL`) and optionally `network_timeout`, `use_tls`, `tls_insecure_skip_verify`.
- **`broker: nats`**
  - The `nats` section must be present with `nats.url` (or `broker_url` or env `NATS_URL`). NATS backend is not yet implemented; `QuickStart` will return an error.
- **`broker: rabbitmq`**
  - Reserved; currently returns “not yet implemented”.

---

## Redis section (`broker: redis`)

```yaml
redis:
  url: redis://localhost:6379/0
  network_timeout: 5
  use_tls: false
  tls_insecure_skip_verify: false
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `url` | string | `REDIS_URL` env or `redis://localhost:6379/0` | Redis connection URL. |
| `network_timeout` | int | 5 | Network timeout in seconds. |
| `use_tls` | bool | false | Use TLS. |
| `tls_insecure_skip_verify` | bool | false | Skip TLS cert verification (insecure). |

---

## NATS section (`broker: nats`)

```yaml
nats:
  url: nats://localhost:4222
  timeout: 5
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `url` | string | `NATS_URL` env or must be set | NATS server URL. Required when broker is nats. |
| `timeout` | int | 5 | Connection timeout in seconds. |

NATS is not yet implemented; this is the expected shape for future use.

---

## Queues

```yaml
queues:
  - [critical, 5]
  - [default, 3]
  - [low, 1]
```

Or long form:

```yaml
queues:
  - name: critical
    weight: 5
  - name: default
    weight: 3
```

- **name**: Queue name used in `Enqueue(workerClass, queue, args...)`.
- **weight**: Relative polling frequency; higher weight means the queue is polled more often. Default: 1.

---

## Example YAML

**Minimal (Redis, defaults)**

```yaml
broker: redis
redis:
  url: redis://localhost:6379/0
```

**Full**

```yaml
broker: redis
# broker_url: optional fallback if redis.url is empty

concurrency: 20
timeout: 15
verbose: true

queues:
  - [critical, 10]
  - [default, 5]
  - [low, 1]

redis:
  url: redis://localhost:6379/0
  network_timeout: 5
  use_tls: false
  tls_insecure_skip_verify: false
```

**Using environment**

```bash
export REDIS_URL=redis://my-host:6379/0
```

Then in YAML you can omit `redis.url` (or set it explicitly); when empty, the loader uses `REDIS_URL`.

---

## Errors from QuickStart

- **File**: `failed to read config file: ...` or `failed to parse config: ...`.
- **Broker**: Unknown `broker` value → `unknown broker '...' (use redis, nats, or rabbitmq)`.
- **NATS with empty URL**: `broker is "nats" but nats.url (or broker_url) is empty`.
- **Redis/NATS**: Broker implementation errors (e.g. invalid URL, connection failure) are returned from `QuickStart`.

Handle these at startup (log and exit or retry as appropriate).
