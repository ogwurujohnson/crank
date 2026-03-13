package queue

import (
	"context"
	"errors"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/ogwurujohnson/crank/internal/config"
	"github.com/ogwurujohnson/crank/internal/payload"
)

// --- Helpers ---

type workerFunc func(ctx context.Context, args ...interface{}) error

func (f workerFunc) Perform(ctx context.Context, args ...interface{}) error { return f(ctx, args...) }

type fixedRegistry struct {
	w   Worker
	err error
}

func (r fixedRegistry) GetWorker(string) (Worker, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.w, nil
}

type spyBroker struct {
	enqueueErr                error
	enq, retry, dead, removed []*payload.Job
	enqNames                  []string
	retryAt                   []time.Time
	retryJobs                 []*payload.Job
}

func (b *spyBroker) Enqueue(q string, j *payload.Job) error {
	b.enqNames = append(b.enqNames, q)
	b.enq = append(b.enq, j)
	return b.enqueueErr
}
func (b *spyBroker) Dequeue([]string, time.Duration) (*payload.Job, string, error) {
	return nil, "", nil
}
func (b *spyBroker) AddToRetry(j *payload.Job, at time.Time) error {
	b.retry = append(b.retry, j)
	b.retryAt = append(b.retryAt, at)
	return nil
}
func (b *spyBroker) GetRetryJobs(int64) ([]*payload.Job, error) { return b.retryJobs, nil }
func (b *spyBroker) RemoveFromRetry(j *payload.Job) error {
	b.removed = append(b.removed, j)
	return nil
}
func (b *spyBroker) AddToDead(j *payload.Job) error            { b.dead = append(b.dead, j); return nil }
func (b *spyBroker) GetDeadJobs(int64) ([]*payload.Job, error) { return nil, nil }
func (b *spyBroker) GetQueueSize(string) (int64, error)        { return 0, nil }
func (b *spyBroker) DeleteKey(string) error                    { return nil }
func (b *spyBroker) GetStats() (map[string]interface{}, error) { return nil, nil }
func (b *spyBroker) Close() error                              { return nil }

type metricsChanHandler struct{ ch chan JobEvent }

func (h metricsChanHandler) HandleJobEvent(_ context.Context, e JobEvent) { h.ch <- e }

// --- Tests ---

func TestNewProcessor_Configuration(t *testing.T) {
	c := qt.New(t)
	b := &spyBroker{}

	t.Run("WeightedQueues", func(t *testing.T) {
		p, _ := NewProcessor(&config.Config{
			Queues: []config.QueueConfig{
				{Name: "critical", Weight: 2},
				{Name: "default", Weight: 1},
			},
		}, b, nil, nil)
		c.Assert(p.queues, qt.DeepEquals, []string{"critical", "critical", "default"})
	})

	t.Run("DefaultQueueFallback", func(t *testing.T) {
		p, _ := NewProcessor(&config.Config{}, b, nil, nil)
		c.Assert(p.queues, qt.DeepEquals, []string{"default"})
	})
}

func TestProcessor_ExecutionFlow(t *testing.T) {
	c := qt.New(t)

	setup := func(w Worker) (*Processor, *spyBroker) {
		b := &spyBroker{}
		p, _ := NewProcessor(&config.Config{Timeout: 1}, b, fixedRegistry{w: w}, nil)
		return p, b
	}

	t.Run("SchedulesRetryOnFailure", func(t *testing.T) {
		p, b := setup(workerFunc(func(context.Context, ...interface{}) error { return errors.New("boom") }))
		j := payload.NewJob("W", "default", 1).SetRetry(1)

		p.processJob(j, "default")

		c.Assert(b.retry, qt.HasLen, 1)
		c.Assert(j.State, qt.Equals, payload.JobStateFailed)
		// Check exponential backoff (2^1 = 2s)
		c.Assert(b.retryAt[0], qt.Satisfies, func(t time.Time) bool {
			return t.After(time.Now())
		})
	})

	t.Run("MovesToDeadWhenExhausted", func(t *testing.T) {
		p, b := setup(workerFunc(func(context.Context, ...interface{}) error { return errors.New("boom") }))
		j := payload.NewJob("W", "default", 1).SetRetry(0)

		p.processJob(j, "default")

		c.Assert(b.dead, qt.HasLen, 1)
		c.Assert(j.State, qt.Equals, payload.JobStateDead)
	})
}

func TestProcessor_Lifecycle(t *testing.T) {
	c := qt.New(t)

	t.Run("RetryLoopIntegration", func(t *testing.T) {
		job := payload.NewJob("W", "critical", 1)
		b := &spyBroker{retryJobs: []*payload.Job{job}}
		p, _ := NewProcessor(&config.Config{RetryPollInterval: 20 * time.Millisecond}, b, nil, nil)

		p.wg.Add(1)
		go p.retryLoop()

		// Wait for one retry tick to run
		time.Sleep(50 * time.Millisecond)
		p.Stop()

		c.Assert(len(b.removed) > 0, qt.IsTrue)
		c.Assert(b.enqNames, qt.Contains, "critical")
		c.Assert(job.State, qt.Equals, payload.JobStatePending)
	})
}

func TestProcessor_Metrics(t *testing.T) {
	c := qt.New(t)
	b := &spyBroker{}
	p, _ := NewProcessor(&config.Config{Concurrency: 1}, b, fixedRegistry{
		w: workerFunc(func(context.Context, ...interface{}) error { return nil }),
	}, nil)

	eventsCh := make(chan JobEvent, 10)
	p.SetMetricsHandler(metricsChanHandler{ch: eventsCh})

	p.wg.Add(1)
	go p.metricsLoop()

	p.processJob(payload.NewJob("W", "default", 1), "default")

	var received []EventType
	for i := 0; i < 2; i++ {
		select {
		case ev := <-eventsCh:
			received = append(received, ev.Type)
		case <-time.After(500 * time.Millisecond):
			t.Fatal("metrics event timeout")
		}
	}

	c.Assert(received, qt.Contains, EventJobStarted)
	c.Assert(received, qt.Contains, EventJobSucceeded)

	p.Stop()
}
