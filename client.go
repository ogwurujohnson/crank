package sidekiq

import (
	"fmt"
	"sync"
)

var (
	globalClient *Client
	clientOnce   sync.Once
)

// Client is the main client for enqueueing jobs
type Client struct {
	redis *RedisClient
}

// NewClient creates a new Sidekiq client
func NewClient(redis *RedisClient) *Client {
	return &Client{redis: redis}
}

// SetGlobalClient sets the global client instance
func SetGlobalClient(client *Client) {
	globalClient = client
}

// GetGlobalClient returns the global client instance
func GetGlobalClient() *Client {
	return globalClient
}

// Enqueue enqueues a job to the specified queue
func (c *Client) Enqueue(workerClass string, queue string, args ...interface{}) (string, error) {
	job := NewJob(workerClass, queue, args...)
	if err := c.redis.Enqueue(queue, job); err != nil {
		return "", fmt.Errorf("failed to enqueue job: %w", err)
	}
	return job.JID, nil
}

// EnqueueWithOptions enqueues a job with custom options
func (c *Client) EnqueueWithOptions(workerClass string, queue string, options *JobOptions, args ...interface{}) (string, error) {
	job := NewJob(workerClass, queue, args...)
	
	if options != nil {
		if options.Retry != nil {
			job.SetRetry(*options.Retry)
		}
		if options.Backtrace != nil {
			job.SetBacktrace(*options.Backtrace)
		}
	}

	if err := c.redis.Enqueue(queue, job); err != nil {
		return "", fmt.Errorf("failed to enqueue job: %w", err)
	}
	return job.JID, nil
}

// JobOptions provides options for job enqueueing
type JobOptions struct {
	Retry     *int
	Backtrace *bool
}

// Enqueue is a convenience function that uses the global client
func Enqueue(workerClass string, queue string, args ...interface{}) (string, error) {
	if globalClient == nil {
		return "", fmt.Errorf("global client not initialized. Call SetGlobalClient first")
	}
	return globalClient.Enqueue(workerClass, queue, args...)
}

// EnqueueWithOptions is a convenience function that uses the global client
func EnqueueWithOptions(workerClass string, queue string, options *JobOptions, args ...interface{}) (string, error) {
	if globalClient == nil {
		return "", fmt.Errorf("global client not initialized. Call SetGlobalClient first")
	}
	return globalClient.EnqueueWithOptions(workerClass, queue, options, args...)
}

