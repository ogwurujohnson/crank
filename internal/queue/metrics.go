package queue

import (
	"context"
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
