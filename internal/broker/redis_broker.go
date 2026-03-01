package broker

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/quest/sidekiq-go/internal/payload"
)

// RedisBrokerConfig holds Redis connection options including TLS
type RedisBrokerConfig struct {
	URL                   string
	Timeout               time.Duration
	UseTLS                bool
	TLSInsecureSkipVerify bool
}

// RedisBroker implements Broker using Redis
type RedisBroker struct {
	client *redis.Client
	ctx    context.Context
}

// NewRedisBroker creates a new Redis broker
func NewRedisBroker(redisURL string, timeout time.Duration) (*RedisBroker, error) {
	return NewRedisBrokerWithConfig(RedisBrokerConfig{
		URL:     redisURL,
		Timeout: timeout,
	})
}

// NewRedisBrokerWithConfig creates a Redis broker with TLS and hardening options.
// It returns an error if the URL is invalid or if Redis is not available (connection verified with Ping).
func NewRedisBrokerWithConfig(cfg RedisBrokerConfig) (*RedisBroker, error) {
	u := strings.TrimSpace(cfg.URL)
	if u == "" {
		return nil, fmt.Errorf("broker not available: Redis URL is empty (set redis.url in config or REDIS_URL)")
	}

	if cfg.UseTLS && !strings.HasPrefix(u, "rediss://") {
		u = strings.Replace(u, "redis://", "rediss://", 1)
	}

	opt, err := redis.ParseURL(u)
	if err != nil {
		return nil, fmt.Errorf("broker not available: invalid Redis URL: %w", err)
	}

	opt.DialTimeout = cfg.Timeout
	opt.ReadTimeout = cfg.Timeout
	opt.WriteTimeout = cfg.Timeout

	if cfg.UseTLS || strings.HasPrefix(u, "rediss://") {
		opt.TLSConfig = &tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: cfg.TLSInsecureSkipVerify,
		}
	}

	client := redis.NewClient(opt)
	ctx := context.Background()

	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("broker not available: Redis unreachable at %q: %w", opt.Addr, err)
	}

	return &RedisBroker{
		client: client,
		ctx:    ctx,
	}, nil
}

// Close closes the Redis connection
func (r *RedisBroker) Close() error {
	return r.client.Close()
}

// Enqueue adds a job to the queue
func (r *RedisBroker) Enqueue(queue string, job *payload.Job) error {
	data, err := job.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize job: %w", err)
	}

	queueKey := fmt.Sprintf("queue:%s", queue)
	if err := r.client.LPush(r.ctx, queueKey, data).Err(); err != nil {
		return fmt.Errorf("failed to enqueue job: %w", err)
	}

	r.client.ZAdd(r.ctx, "stat:processed", &redis.Z{
		Score:  float64(time.Now().Unix()),
		Member: job.JID,
	})

	return nil
}

// Ack is a no-op for Redis since BRPOP is atomic
func (r *RedisBroker) Ack(job *payload.Job) error {
	return nil
}

// Dequeue removes and returns a job from the queue
func (r *RedisBroker) Dequeue(queues []string, timeout time.Duration) (*payload.Job, string, error) {
	queueKeys := make([]string, len(queues))
	for i, q := range queues {
		queueKeys[i] = fmt.Sprintf("queue:%s", q)
	}

	result, err := r.client.BRPop(r.ctx, timeout, queueKeys...).Result()
	if err == redis.Nil {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", fmt.Errorf("failed to dequeue: %w", err)
	}

	if len(result) < 2 {
		return nil, "", fmt.Errorf("invalid BRPop result")
	}

	queueName := result[0][6:]
	job, err := payload.FromJSON([]byte(result[1]))
	if err != nil {
		return nil, "", fmt.Errorf("failed to deserialize job: %w", err)
	}

	return job, queueName, nil
}

// AddToRetry adds a job to the retry set
func (r *RedisBroker) AddToRetry(job *payload.Job, retryAt time.Time) error {
	data, err := job.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize job: %w", err)
	}

	return r.client.ZAdd(r.ctx, "retry", &redis.Z{
		Score:  float64(retryAt.Unix()),
		Member: data,
	}).Err()
}

// GetRetryJobs returns jobs ready to be retried
func (r *RedisBroker) GetRetryJobs(limit int64) ([]*payload.Job, error) {
	now := float64(time.Now().Unix())
	result, err := r.client.ZRangeByScore(r.ctx, "retry", &redis.ZRangeBy{
		Min: "0", Max: fmt.Sprintf("%.0f", now), Offset: 0, Count: limit,
	}).Result()

	if err != nil {
		return nil, fmt.Errorf("failed to get retry jobs: %w", err)
	}

	jobs := make([]*payload.Job, 0, len(result))
	for _, data := range result {
		job, err := payload.FromJSON([]byte(data))
		if err != nil {
			continue
		}
		jobs = append(jobs, job)
	}

	return jobs, nil
}

// RemoveFromRetry removes a job from the retry set
func (r *RedisBroker) RemoveFromRetry(job *payload.Job) error {
	data, err := job.ToJSON()
	if err != nil {
		return err
	}
	return r.client.ZRem(r.ctx, "retry", data).Err()
}

// AddToDead adds a job to the dead set
func (r *RedisBroker) AddToDead(job *payload.Job) error {
	data, err := job.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize job: %w", err)
	}

	return r.client.ZAdd(r.ctx, "dead", &redis.Z{
		Score:  float64(time.Now().Unix()),
		Member: data,
	}).Err()
}

// GetQueueSize returns the size of a queue
func (r *RedisBroker) GetQueueSize(queue string) (int64, error) {
	return r.client.LLen(r.ctx, "queue:"+queue).Result()
}

// DeleteKey deletes a key from Redis
func (r *RedisBroker) DeleteKey(key string) error {
	return r.client.Del(r.ctx, key).Err()
}

// GetStats returns queue statistics
func (r *RedisBroker) GetStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	processed, _ := r.client.ZCard(r.ctx, "stat:processed").Result()
	stats["processed"] = processed

	retry, _ := r.client.ZCard(r.ctx, "retry").Result()
	stats["retry"] = retry

	dead, _ := r.client.ZCard(r.ctx, "dead").Result()
	stats["dead"] = dead

	queues := []string{"critical", "default", "low"}
	queueSizes := make(map[string]int64)
	for _, q := range queues {
		size, _ := r.GetQueueSize(q)
		queueSizes[q] = size
	}
	stats["queues"] = queueSizes

	return stats, nil
}
