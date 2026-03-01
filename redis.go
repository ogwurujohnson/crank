package sidekiq

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

// RedisClient wraps the Redis client
type RedisClient struct {
	client *redis.Client
	ctx    context.Context
}

// NewRedisClient creates a new Redis client
func NewRedisClient(url string, timeout time.Duration) (*RedisClient, error) {
	opt, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Redis URL: %w", err)
	}

	opt.DialTimeout = timeout
	opt.ReadTimeout = timeout
	opt.WriteTimeout = timeout

	client := redis.NewClient(opt)
	ctx := context.Background()

	// Test connection
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisClient{
		client: client,
		ctx:    ctx,
	}, nil
}

// Close closes the Redis connection
func (r *RedisClient) Close() error {
	return r.client.Close()
}

// Enqueue adds a job to the queue
func (r *RedisClient) Enqueue(queue string, job *Job) error {
	data, err := job.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize job: %w", err)
	}

	queueKey := fmt.Sprintf("queue:%s", queue)
	if err := r.client.LPush(r.ctx, queueKey, data).Err(); err != nil {
		return fmt.Errorf("failed to enqueue job: %w", err)
	}

	// Add to processed set for stats
	r.client.ZAdd(r.ctx, "stat:processed", &redis.Z{
		Score:  float64(time.Now().Unix()),
		Member: job.JID,
	})

	return nil
}

// Dequeue removes and returns a job from the queue
func (r *RedisClient) Dequeue(queues []string, timeout time.Duration) (*Job, string, error) {
	// Build queue keys
	queueKeys := make([]string, len(queues))
	for i, q := range queues {
		queueKeys[i] = fmt.Sprintf("queue:%s", q)
	}

	// Blocking pop from multiple queues
	result, err := r.client.BRPop(r.ctx, timeout, queueKeys...).Result()
	if err == redis.Nil {
		return nil, "", nil // Timeout, no job available
	}
	if err != nil {
		return nil, "", fmt.Errorf("failed to dequeue: %w", err)
	}

	if len(result) < 2 {
		return nil, "", fmt.Errorf("invalid BRPop result")
	}

	queueKey := result[0]
	queueName := queueKey[6:] // Remove "queue:" prefix
	data := result[1]

	job, err := FromJSON([]byte(data))
	if err != nil {
		return nil, "", fmt.Errorf("failed to deserialize job: %w", err)
	}

	return job, queueName, nil
}

// AddToRetry adds a job to the retry set
func (r *RedisClient) AddToRetry(job *Job, retryAt time.Time) error {
	data, err := job.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize job: %w", err)
	}

	score := float64(retryAt.Unix())
	if err := r.client.ZAdd(r.ctx, "retry", &redis.Z{
		Score:  score,
		Member: data,
	}).Err(); err != nil {
		return fmt.Errorf("failed to add to retry: %w", err)
	}

	return nil
}

// GetRetryJobs returns jobs ready to be retried
func (r *RedisClient) GetRetryJobs(limit int64) ([]*Job, error) {
	now := float64(time.Now().Unix())
	result, err := r.client.ZRangeByScore(r.ctx, "retry", &redis.ZRangeBy{
		Min:    "0",
		Max:    fmt.Sprintf("%.0f", now),
		Offset: 0,
		Count:  limit,
	}).Result()

	if err != nil {
		return nil, fmt.Errorf("failed to get retry jobs: %w", err)
	}

	jobs := make([]*Job, 0, len(result))
	for _, data := range result {
		job, err := FromJSON([]byte(data))
		if err != nil {
			continue // Skip invalid jobs
		}
		jobs = append(jobs, job)
	}

	return jobs, nil
}

// RemoveFromRetry removes a job from the retry set
func (r *RedisClient) RemoveFromRetry(job *Job) error {
	data, err := job.ToJSON()
	if err != nil {
		return err
	}

	return r.client.ZRem(r.ctx, "retry", data).Err()
}

// AddToDead adds a job to the dead set
func (r *RedisClient) AddToDead(job *Job) error {
	data, err := job.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize job: %w", err)
	}

	now := float64(time.Now().Unix())
	if err := r.client.ZAdd(r.ctx, "dead", &redis.Z{
		Score:  now,
		Member: data,
	}).Err(); err != nil {
		return fmt.Errorf("failed to add to dead: %w", err)
	}

	return nil
}

// GetQueueSize returns the size of a queue
func (r *RedisClient) GetQueueSize(queue string) (int64, error) {
	queueKey := fmt.Sprintf("queue:%s", queue)
	return r.client.LLen(r.ctx, queueKey).Result()
}

// DeleteKey deletes a key from Redis
func (r *RedisClient) DeleteKey(key string) error {
	return r.client.Del(r.ctx, key).Err()
}

// GetStats returns queue statistics
func (r *RedisClient) GetStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Get processed count
	processed, _ := r.client.ZCard(r.ctx, "stat:processed").Result()
	stats["processed"] = processed

	// Get retry count
	retry, _ := r.client.ZCard(r.ctx, "retry").Result()
	stats["retry"] = retry

	// Get dead count
	dead, _ := r.client.ZCard(r.ctx, "dead").Result()
	stats["dead"] = dead

	// Get queue sizes
	queues := []string{"critical", "default", "low"}
	queueSizes := make(map[string]int64)
	for _, q := range queues {
		size, _ := r.GetQueueSize(q)
		queueSizes[q] = size
	}
	stats["queues"] = queueSizes

	return stats, nil
}

