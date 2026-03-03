package queue

import (
	"context"
	"sync"
	"time"

	"github.com/ogwurujohnson/crank/internal/payload"
)

type EventType string

const (
	EventJobStarted        EventType = "job_started"
	EventJobSucceeded      EventType = "job_succeeded"
	EventJobFailed         EventType = "job_failed"
	EventJobRetryScheduled EventType = "job_retry_scheduled"
	EventJobMovedToDead    EventType = "job_moved_to_dead"
)

type JobEvent struct {
	Type     EventType
	Job      *payload.Job
	Queue    string
	Duration time.Duration
	Err      error
}

type MetricsHandler interface {
	HandleJobEvent(ctx context.Context, e JobEvent)
}

// InMemoryMetrics is a simple MetricsHandler implementation that keeps
// running totals of job outcomes in memory.
type InMemoryMetrics struct {
	mu sync.RWMutex

	processedTotal int64
	failedTotal    int64
}

func (m *InMemoryMetrics) HandleJobEvent(_ context.Context, e JobEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch e.Type {
	case EventJobSucceeded:
		m.processedTotal++
	case EventJobFailed:
		m.failedTotal++
	}
}

func (m *InMemoryMetrics) ProcessedTotal() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.processedTotal
}

func (m *InMemoryMetrics) FailedTotal() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.failedTotal
}
