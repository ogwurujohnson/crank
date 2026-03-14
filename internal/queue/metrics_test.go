package queue

import (
	"context"
	"testing"

	"github.com/ogwurujohnson/crank/internal/payload"
)

func TestInMemoryMetrics_CountsEvents(t *testing.T) {
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

	if got := metrics.ProcessedTotal(); got != 1 {
		t.Errorf("ProcessedTotal() = %d, want 1", got)
	}
	if got := metrics.FailedTotal(); got != 2 {
		t.Errorf("FailedTotal() = %d, want 2", got)
	}
}
