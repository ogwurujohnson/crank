package broker

import (
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	qt "github.com/frankban/quicktest"
	"github.com/ogwurujohnson/crank/internal/payload"
)

func TestNewRedisBrokerWithConfig_EmptyURL(t *testing.T) {
	c := qt.New(t)

	_, err := NewRedisBrokerWithConfig(RedisBrokerConfig{})
	c.Assert(err, qt.ErrorMatches, `broker not available: Redis URL is empty .*`)
}

func TestRedisBroker_EnqueueDequeue(t *testing.T) {
	c := qt.New(t)

	s := miniredis.RunT(t)
	r, err := NewRedisBroker(fmt.Sprintf("redis://%s/0", s.Addr()), time.Second)
	c.Assert(err, qt.IsNil)
	t.Cleanup(func() { _ = r.Close() })

	j := payload.NewJob("W", "default", 1, "two")
	c.Assert(r.Enqueue("default", j), qt.IsNil)

	got, q, err := r.Dequeue([]string{"default"}, time.Second)
	c.Assert(err, qt.IsNil)
	c.Assert(q, qt.Equals, "default")
	c.Assert(got.JID, qt.Equals, j.JID)
	c.Assert(got.Class, qt.Equals, j.Class)
	c.Assert(got.Queue, qt.Equals, j.Queue)
}

func TestRedisBroker_RetryLifecycle(t *testing.T) {
	c := qt.New(t)

	s := miniredis.RunT(t)
	r, err := NewRedisBroker(fmt.Sprintf("redis://%s/0", s.Addr()), time.Second)
	c.Assert(err, qt.IsNil)
	t.Cleanup(func() { _ = r.Close() })

	j := payload.NewJob("W", "default", 1)
	c.Assert(r.AddToRetry(j, time.Now().Add(-1*time.Second)), qt.IsNil)

	jobs, err := r.GetRetryJobs(10)
	c.Assert(err, qt.IsNil)
	c.Assert(len(jobs), qt.Equals, 1)
	c.Assert(jobs[0].JID, qt.Equals, j.JID)

	c.Assert(r.RemoveFromRetry(jobs[0]), qt.IsNil)
	jobs, err = r.GetRetryJobs(10)
	c.Assert(err, qt.IsNil)
	c.Assert(len(jobs), qt.Equals, 0)
}

func TestRedisBroker_DeadLifecycle(t *testing.T) {
	c := qt.New(t)

	s := miniredis.RunT(t)
	r, err := NewRedisBroker(fmt.Sprintf("redis://%s/0", s.Addr()), time.Second)
	c.Assert(err, qt.IsNil)
	t.Cleanup(func() { _ = r.Close() })

	j := payload.NewJob("W", "default", 1)
	c.Assert(r.AddToDead(j), qt.IsNil)

	jobs, err := r.GetDeadJobs(10)
	c.Assert(err, qt.IsNil)
	c.Assert(len(jobs) >= 1, qt.IsTrue)
	c.Assert(jobs[0].JID, qt.Equals, j.JID)
}

func TestRedisBroker_GetStats(t *testing.T) {
	c := qt.New(t)

	s := miniredis.RunT(t)
	r, err := NewRedisBroker(fmt.Sprintf("redis://%s/0", s.Addr()), time.Second)
	c.Assert(err, qt.IsNil)
	t.Cleanup(func() { _ = r.Close() })

	c.Assert(r.Enqueue("default", payload.NewJob("W", "default", 1)), qt.IsNil)
	c.Assert(r.AddToRetry(payload.NewJob("W", "default", 2), time.Now().Add(-1*time.Second)), qt.IsNil)
	c.Assert(r.AddToDead(payload.NewJob("W", "default", 3)), qt.IsNil)

	stats, err := r.GetStats()
	c.Assert(err, qt.IsNil)

	c.Assert(stats["processed"], qt.Equals, int64(1))
	c.Assert(stats["retry"], qt.Equals, int64(1))
	c.Assert(stats["dead"], qt.Equals, int64(1))

	qs, ok := stats["queues"].(map[string]int64)
	c.Assert(ok, qt.IsTrue)
	c.Assert(qs["default"], qt.Equals, int64(1))
}
