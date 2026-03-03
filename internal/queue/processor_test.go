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
	enqueueErr error

	enqQ []string
	enq  []*payload.Job

	retryAt []time.Time
	retry   []*payload.Job

	dead []*payload.Job

	retryJobs []*payload.Job
	removed   []*payload.Job
}

func (b *spyBroker) Enqueue(q string, j *payload.Job) error {
	b.enqQ = append(b.enqQ, q)
	b.enq = append(b.enq, j)
	return b.enqueueErr
}
func (b *spyBroker) Dequeue([]string, time.Duration) (*payload.Job, string, error) {
	return nil, "", nil
}
func (b *spyBroker) Ack(*payload.Job) error { return nil }
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
func (b *spyBroker) AddToDead(j *payload.Job) error {
	b.dead = append(b.dead, j)
	return nil
}
func (b *spyBroker) GetDeadJobs(int64) ([]*payload.Job, error) { return nil, nil }
func (b *spyBroker) GetQueueSize(string) (int64, error)        { return 0, nil }
func (b *spyBroker) DeleteKey(string) error                    { return nil }
func (b *spyBroker) GetStats() (map[string]interface{}, error) { return nil, nil }
func (b *spyBroker) Close() error                              { return nil }

func TestNewProcessor_QueuesWeightedDefault(t *testing.T) {
	c := qt.New(t)

	b := &spyBroker{}
	p, err := NewProcessor(&config.Config{
		Concurrency: 1,
		Queues: []config.QueueConfig{
			{Name: "critical", Weight: 2},
			{Name: "default", Weight: 1},
		},
		Timeout: 1,
	}, b, nil)
	c.Assert(err, qt.IsNil)
	c.Assert(p.queues, qt.DeepEquals, []string{"critical", "critical", "default"})

	p, err = NewProcessor(&config.Config{Concurrency: 1, Timeout: 1}, b, nil)
	c.Assert(err, qt.IsNil)
	c.Assert(p.queues, qt.DeepEquals, []string{"default"})
}

func TestProcessor_processJob_SchedulesRetryOnWorkerError(t *testing.T) {
	c := qt.New(t)

	b := &spyBroker{}
	p, err := NewProcessor(&config.Config{Concurrency: 1, Timeout: 1}, b, fixedRegistry{
		w: workerFunc(func(context.Context, ...interface{}) error { return errors.New("boom") }),
	})
	c.Assert(err, qt.IsNil)

	j := payload.NewJob("W", "default", 1).SetRetry(1)
	t0 := time.Now()
	p.processJob(j, "default")

	c.Assert(len(b.retry), qt.Equals, 1)
	c.Assert(b.retry[0].JID, qt.Equals, j.JID)
	c.Assert(b.retryAt[0].Sub(t0) >= 2*time.Second, qt.IsTrue)
	c.Assert(b.retryAt[0].Sub(t0) < 3*time.Second, qt.IsTrue)
	c.Assert(len(b.dead), qt.Equals, 0)
}

func TestProcessor_processJob_MovesToDeadWhenRetryExceeded(t *testing.T) {
	c := qt.New(t)

	b := &spyBroker{}
	p, err := NewProcessor(&config.Config{Concurrency: 1, Timeout: 1}, b, fixedRegistry{
		w: workerFunc(func(context.Context, ...interface{}) error { return errors.New("boom") }),
	})
	c.Assert(err, qt.IsNil)

	j := payload.NewJob("W", "default", 1).SetRetry(0)
	p.processJob(j, "default")

	c.Assert(len(b.retry), qt.Equals, 0)
	c.Assert(len(b.dead), qt.Equals, 1)
	c.Assert(b.dead[0].JID, qt.Equals, j.JID)
}

func TestProcessor_processRetries_Reenqueues(t *testing.T) {
	c := qt.New(t)

	b := &spyBroker{
		retryJobs: []*payload.Job{
			payload.NewJob("W", "critical", 1),
		},
	}
	p, err := NewProcessor(&config.Config{Concurrency: 1, Timeout: 1}, b, nil)
	c.Assert(err, qt.IsNil)

	p.processRetries()

	c.Assert(len(b.removed), qt.Equals, 1)
	c.Assert(b.removed[0].JID, qt.Equals, b.retryJobs[0].JID)

	c.Assert(len(b.enq), qt.Equals, 1)
	c.Assert(b.enqQ[0], qt.Equals, "critical")
	c.Assert(b.enq[0].JID, qt.Equals, b.retryJobs[0].JID)
}

func TestProcessor_processRetries_ReaddsOnEnqueueError(t *testing.T) {
	c := qt.New(t)

	j := payload.NewJob("W", "critical", 1)
	b := &spyBroker{
		enqueueErr: errors.New("nope"),
		retryJobs:  []*payload.Job{j},
	}
	p, err := NewProcessor(&config.Config{Concurrency: 1, Timeout: 1}, b, nil)
	c.Assert(err, qt.IsNil)

	t0 := time.Now()
	p.processRetries()

	c.Assert(len(b.retry), qt.Equals, 1)
	c.Assert(b.retry[0].JID, qt.Equals, j.JID)
	c.Assert(b.retryAt[0].Sub(t0) >= time.Minute, qt.IsTrue)
	c.Assert(b.retryAt[0].Sub(t0) < 2*time.Minute, qt.IsTrue)
}
