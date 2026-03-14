package broker

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/ogwurujohnson/crank/internal/payload"
)

type RedisBrokerConfig struct {
	URL                   string
	Timeout               time.Duration
	UseTLS                bool
	TLSInsecureSkipVerify bool
}

type RedisBroker struct {
	client *redis.Client
	ctx    context.Context
}

func NewRedisBroker(redisURL string, timeout time.Duration) (*RedisBroker, error) {
	return NewRedisBrokerWithConfig(RedisBrokerConfig{
		URL:     redisURL,
		Timeout: timeout,
	})
}

func NewRedisBrokerWithConfig(cfg RedisBrokerConfig) (*RedisBroker, error) {
	trimmedURL := strings.TrimSpace(cfg.URL)
	if trimmedURL == "" {
		return nil, fmt.Errorf("broker not available: Redis URL is empty (set redis.url in config or REDIS_URL)")
	}

	if cfg.UseTLS && !strings.HasPrefix(trimmedURL, "rediss://") {
		trimmedURL = strings.Replace(trimmedURL, "redis://", "rediss://", 1)
	}

	opt, err := redis.ParseURL(trimmedURL)
	if err != nil {
		return nil, fmt.Errorf("broker not available: invalid Redis URL: %w", err)
	}

	opt.DialTimeout = cfg.Timeout
	opt.ReadTimeout = cfg.Timeout
	opt.WriteTimeout = cfg.Timeout

	if cfg.UseTLS || strings.HasPrefix(trimmedURL, "rediss://") {
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

func (r *RedisBroker) Close() error {
	return r.client.Close()
}

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

func (r *RedisBroker) Dequeue(queues []string, timeout time.Duration) (*payload.Job, string, error) {
	queueKeys := make([]string, len(queues))
	for queueIndex, queueName := range queues {
		queueKeys[queueIndex] = fmt.Sprintf("queue:%s", queueName)
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

func (r *RedisBroker) GetRetryJobs(limit int64) ([]*payload.Job, error) {
	if limit <= 0 {
		limit = 1
	}
	if limit > 10000 {
		limit = 10000
	}
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

func (r *RedisBroker) RemoveFromRetry(job *payload.Job) error {
	data, err := job.ToJSON()
	if err != nil {
		return err
	}
	return r.client.ZRem(r.ctx, "retry", data).Err()
}

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

func (r *RedisBroker) GetDeadJobs(limit int64) ([]*payload.Job, error) {
	if limit <= 0 {
		limit = 1
	}
	if limit > 10000 {
		limit = 10000
	}
	result, err := r.client.ZRevRange(r.ctx, "dead", 0, limit-1).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get dead jobs: %w", err)
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

func (r *RedisBroker) GetQueueSize(queue string) (int64, error) {
	return r.client.LLen(r.ctx, "queue:"+queue).Result()
}

func (r *RedisBroker) DeleteKey(key string) error {
	return r.client.Del(r.ctx, key).Err()
}

func (r *RedisBroker) GetStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	processed, _ := r.client.ZCard(r.ctx, "stat:processed").Result()
	stats["processed"] = processed

	retry, _ := r.client.ZCard(r.ctx, "retry").Result()
	stats["retry"] = retry

	dead, _ := r.client.ZCard(r.ctx, "dead").Result()
	stats["dead"] = dead

	queueSizes := make(map[string]int64)
	keys, err := r.client.Keys(r.ctx, "queue:*").Result()
	if err != nil {
		stats["queues"] = queueSizes
		return stats, nil
	}
	for _, key := range keys {
		if len(key) > 6 {
			name := key[6:]
			size, _ := r.client.LLen(r.ctx, key).Result()
			queueSizes[name] = size
		}
	}
	stats["queues"] = queueSizes

	return stats, nil
}
