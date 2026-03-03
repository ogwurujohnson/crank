package queue

import (
	"context"
	"errors"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/ogwurujohnson/crank/internal/payload"
)

func TestCircuitBreaker_Allow_Closed(t *testing.T) {
	c := qt.New(t)
	b := NewCircuitBreaker(BreakerConfig{
		FailureThreshold: 2,
		Window:           time.Minute,
		ResetTimeout:     time.Second,
	})
	c.Assert(b.Allow("A"), qt.IsTrue)
	c.Assert(b.Allow("A"), qt.IsTrue)
}

func TestCircuitBreaker_OpensAfterThreshold(t *testing.T) {
	c := qt.New(t)
	b := NewCircuitBreaker(BreakerConfig{
		FailureThreshold: 2,
		Window:           time.Minute,
		ResetTimeout:     100 * time.Millisecond,
	})
	b.RecordFailure("A")
	c.Assert(b.Allow("A"), qt.IsTrue)
	b.RecordFailure("A")
	c.Assert(b.Allow("A"), qt.IsFalse)
	c.Assert(b.IsOpen("A"), qt.IsTrue)
}

func TestCircuitBreaker_HalfOpen_AllowsOneThenSuccess(t *testing.T) {
	c := qt.New(t)
	b := NewCircuitBreaker(BreakerConfig{
		FailureThreshold: 1,
		Window:           time.Minute,
		ResetTimeout:     10 * time.Millisecond,
	})
	b.RecordFailure("A")
	c.Assert(b.Allow("A"), qt.IsFalse)
	time.Sleep(20 * time.Millisecond)
	c.Assert(b.Allow("A"), qt.IsTrue)
	c.Assert(b.Allow("A"), qt.IsFalse)
	b.RecordSuccess("A")
	c.Assert(b.Allow("A"), qt.IsTrue)
}

func TestCircuitBreaker_HalfOpen_FailureReopens(t *testing.T) {
	c := qt.New(t)
	b := NewCircuitBreaker(BreakerConfig{
		FailureThreshold: 1,
		Window:           time.Minute,
		ResetTimeout:     10 * time.Millisecond,
	})
	b.RecordFailure("A")
	time.Sleep(20 * time.Millisecond)
	c.Assert(b.Allow("A"), qt.IsTrue)
	b.RecordFailure("A")
	c.Assert(b.Allow("A"), qt.IsFalse)
}

func TestCircuitBreaker_SuccessClearsFailuresInClosed(t *testing.T) {
	c := qt.New(t)
	b := NewCircuitBreaker(BreakerConfig{
		FailureThreshold: 2,
		Window:           time.Minute,
		ResetTimeout:     time.Second,
	})
	b.RecordFailure("A")
	b.RecordSuccess("A")
	b.RecordFailure("A")
	c.Assert(b.Allow("A"), qt.IsTrue)
}

func TestBreakerMiddleware_RecordsOutcome(t *testing.T) {
	c := qt.New(t)
	b := NewCircuitBreaker(BreakerConfig{
		FailureThreshold: 1,
		Window:           time.Minute,
		ResetTimeout:     time.Millisecond,
	})
	mw := BreakerMiddleware(b)
	h := mw(func(ctx context.Context, job *payload.Job) error { return nil })
	job := payload.NewJob("W", "q")
	err := h(nil, job)
	c.Assert(err, qt.IsNil)
	c.Assert(b.Allow("W"), qt.IsTrue)

	h2 := mw(func(ctx context.Context, job *payload.Job) error { return errFake })
	err = h2(nil, job)
	c.Assert(err, qt.Equals, errFake)
	// One failure opens (threshold 1)
	c.Assert(b.Allow("W"), qt.IsFalse)
}

var errFake = errors.New("fake")
