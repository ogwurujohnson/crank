package sdk

import (
	"fmt"

	"github.com/quest/crank/internal/broker"
	"github.com/quest/crank/internal/payload"
)

var (
	globalClient *Client
)

// Client is the thin client for enqueueing jobs to the broker
type Client struct {
	broker broker.Broker
}

// NewClient creates a new Crank client
func NewClient(b broker.Broker) *Client {
	return &Client{broker: b}
}

// SetGlobalClient sets the global client instance
func SetGlobalClient(c *Client) {
	globalClient = c
}

// GetGlobalClient returns the global client instance
func GetGlobalClient() *Client {
	return globalClient
}

// Enqueue enqueues a job to the specified queue
func (c *Client) Enqueue(workerClass string, queue string, args ...interface{}) (string, error) {
	job := payload.NewJob(workerClass, queue, args...)
	if err := c.broker.Enqueue(queue, job); err != nil {
		return "", fmt.Errorf("failed to enqueue job: %w", err)
	}
	return job.JID, nil
}

// EnqueueWithOptions enqueues a job with custom options
func (c *Client) EnqueueWithOptions(workerClass string, queue string, options *payload.JobOptions, args ...interface{}) (string, error) {
	job := payload.NewJob(workerClass, queue, args...)

	if options != nil {
		if options.Retry != nil {
			job.SetRetry(*options.Retry)
		}
		if options.Backtrace != nil {
			job.SetBacktrace(*options.Backtrace)
		}
	}

	if err := c.broker.Enqueue(queue, job); err != nil {
		return "", fmt.Errorf("failed to enqueue job: %w", err)
	}
	return job.JID, nil
}

// EnqueueGlobal is a convenience that uses the global client
func EnqueueGlobal(workerClass string, queue string, args ...interface{}) (string, error) {
	if globalClient == nil {
		return "", fmt.Errorf("global client not initialized. Call SetGlobalClient first")
	}
	return globalClient.Enqueue(workerClass, queue, args...)
}

// EnqueueWithOptionsGlobal is a convenience that uses the global client
func EnqueueWithOptionsGlobal(workerClass string, queue string, options *payload.JobOptions, args ...interface{}) (string, error) {
	if globalClient == nil {
		return "", fmt.Errorf("global client not initialized. Call SetGlobalClient first")
	}
	return globalClient.EnqueueWithOptions(workerClass, queue, options, args...)
}
