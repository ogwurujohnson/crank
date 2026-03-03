package queue

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/ogwurujohnson/crank/internal/payload"
)

func TestInMemoryMetrics_CountsEvents(t *testing.T) {
	c := qt.New(t)

	metrics := &InMemoryMetrics{}

	metrics.HandleJobEvent(context.Background(), JobEvent{
		Type: EventJobSucceeded,
		Job:  payload.NewJob("W", "default"),
	})
	metrics.HandleJobEvent(context.Background(), JobEvent{
		Type: EventJobFailed,
		Job:  payload.NewJob("W", "default"),
	})
	metrics.HandleJobEvent(context.Background(), JobEvent{
		Type: EventJobFailed,
		Job:  payload.NewJob("W", "default"),
	})
	metrics.HandleJobEvent(context.Background(), JobEvent{
		Type: EventJobMovedToDead,
		Job:  payload.NewJob("W", "default"),
	})

	c.Assert(metrics.ProcessedTotal(), qt.Equals, int64(1))
	c.Assert(metrics.FailedTotal(), qt.Equals, int64(2))
}

