package client

import (
	"fmt"
	"sync/atomic"

	"github.com/ogwurujohnson/crank/internal/broker"
	"github.com/ogwurujohnson/crank/internal/payload"
)

var globalClient atomic.Pointer[Client]

type Client struct {
	broker broker.Broker
}

func New(b broker.Broker) *Client {
	return &Client{broker: b}
}

func SetGlobal(c *Client) {
	globalClient.Store(c)
}

func GetGlobal() *Client {
	return globalClient.Load()
}

func (c *Client) Enqueue(workerClass string, queue string, args ...interface{}) (string, error) {
	job := payload.NewJob(workerClass, queue, args...)
	if err := c.broker.Enqueue(queue, job); err != nil {
		return "", fmt.Errorf("failed to enqueue job: %w", err)
	}
	return job.JID, nil
}

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

func EnqueueGlobal(workerClass string, queue string, args ...interface{}) (string, error) {
	c := globalClient.Load()
	if c == nil {
		return "", fmt.Errorf("global client not initialized. Call SetGlobalClient first")
	}
	return c.Enqueue(workerClass, queue, args...)
}

func EnqueueWithOptionsGlobal(workerClass string, queue string, options *payload.JobOptions, args ...interface{}) (string, error) {
	c := globalClient.Load()
	if c == nil {
		return "", fmt.Errorf("global client not initialized. Call SetGlobalClient first")
	}
	return c.EnqueueWithOptions(workerClass, queue, options, args...)
}
