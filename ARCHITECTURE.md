# Architecture Overview

## Core Components

### 1. Job (`job.go`)
- Represents a background job with metadata
- Serializes/deserializes to/from JSON
- Tracks retry count, queue, and job ID (JID)

### 2. Worker (`worker.go`)
- Interface: `Worker.Perform(ctx, args...)`
- Registry: Thread-safe map of worker classes
- Workers are registered before processing starts

### 3. Redis Client (`redis.go`)
- Wraps go-redis client
- Handles job enqueueing/dequeueing
- Manages retry and dead job sets (sorted sets)
- Provides queue statistics

### 4. Processor (`processor.go`)
- Main processing engine
- Worker pool with configurable concurrency
- Weighted queue polling
- Automatic retry processing
- Graceful shutdown support

### 5. Client (`client.go`)
- Enqueues jobs to Redis
- Supports job options (retry, backtrace)
- Global client for convenience

### 6. Configuration (`config.go`)
- YAML-based configuration
- Supports array and map queue formats
- Environment variable support (REDIS_URL)

### 7. Queue Management (`queue.go`)
- Queue operations (size, clear)
- Statistics aggregation

### 8. Middleware (`middleware.go`)
- Chain-based middleware system
- Executes before/after job processing
- Useful for logging, metrics, error handling

### 9. Web UI (`web/web.go`)
- HTTP interface for monitoring
- Queue statistics
- Queue management (clear)
- Real-time updates via JavaScript

## Data Flow

### Enqueueing a Job

```
Client.Enqueue()
  → Job.ToJSON()
  → RedisClient.Enqueue()
  → Redis LPUSH queue:name [job_json]
```

### Processing a Job

```
Processor.worker()
  → RedisClient.Dequeue() (BRPOP)
  → Processor.processJob()
  → MiddlewareChain.Execute()
  → Worker.Perform()
  → Success: Done
  → Failure: RedisClient.AddToRetry() or AddToDead()
```

### Retry Processing

```
Processor.retryProcessor() (periodic ticker)
  → RedisClient.GetRetryJobs() (ZRANGEBYSCORE)
  → For each ready job:
    → Remove from retry set
    → Re-enqueue to original queue
```

## Redis Data Structures

### Queues
- `queue:{name}` - List of pending jobs
- Jobs are stored as JSON strings

### Retry Set
- `retry` - Sorted set (ZSET)
- Score: Unix timestamp when job should be retried
- Member: Job JSON

### Dead Set
- `dead` - Sorted set (ZSET)
- Score: Unix timestamp when job died
- Member: Job JSON

### Statistics
- `stat:processed` - Sorted set tracking processed jobs
- Used for counting processed jobs

## Concurrency Model

- **Goroutines**: One per worker slot (concurrency setting)
- **Worker Pool**: Channel-based semaphore limits concurrent jobs
- **Queue Polling**: Blocking BRPOP with timeout
- **Retry Processing**: Separate goroutine with periodic ticker

## Queue Weighting

Queues are weighted by repeating queue names in the polling list:
- `[critical, 5]` → appears 5 times in polling list
- `[default, 3]` → appears 3 times
- `[low, 1]` → appears 1 time

BRPOP checks queues in order, so higher-weighted queues are checked more frequently.

## Error Handling

1. **Worker Not Found**: Job moved to dead set immediately
2. **Job Execution Error**: 
   - If retry count < max: Add to retry set with exponential backoff
   - If retry count >= max: Add to dead set
3. **Redis Errors**: Logged, job processing continues
4. **Timeout**: Context timeout cancels job execution

## Performance Considerations

1. **Connection Pooling**: Redis client handles connection pooling
2. **JSON Serialization**: Efficient but consider msgpack for higher throughput
3. **Goroutine Overhead**: Minimal, Go runtime handles efficiently
4. **Queue Polling**: BRPOP is efficient, blocks until job available
5. **Retry Processing**: Batched retrieval (100 jobs at a time)

## Scalability

- **Horizontal**: Run multiple Sidekiq processes (different machines/containers)
- **Vertical**: Increase concurrency per process (watch DB connection pool)
- **Queue-based**: Scale specific queues by running dedicated processes

## Security Considerations

1. **Redis Access**: Use Redis ACLs or password authentication
2. **Web UI**: Should be protected with authentication middleware
3. **Job Arguments**: Sensitive data should be encrypted or referenced by ID
4. **Network**: Use TLS for Redis connections in production

