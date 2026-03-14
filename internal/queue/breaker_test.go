package queue

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ogwurujohnson/crank/internal/payload"
)

func TestCircuitBreaker_Allow_Closed(t *testing.T) {
	b := NewCircuitBreaker(BreakerConfig{
		FailureThreshold: 2,
		Window:           time.Minute,
		ResetTimeout:     time.Second,
	})
	if !b.Allow("A") {
		t.Error("Allow(A) = false, want true")
	}
	if !b.Allow("A") {
		t.Error("Allow(A) = false, want true")
	}
}

func TestCircuitBreaker_OpensAfterThreshold(t *testing.T) {
	b := NewCircuitBreaker(BreakerConfig{
		FailureThreshold: 2,
		Window:           time.Minute,
		ResetTimeout:     100 * time.Millisecond,
	})
	b.RecordFailure("A")
	if !b.Allow("A") {
		t.Error("Allow(A) after 1 failure = false, want true")
	}
	b.RecordFailure("A")
	if b.Allow("A") {
		t.Error("Allow(A) after 2 failures = true, want false")
	}
	if !b.IsOpen("A") {
		t.Error("IsOpen(A) = false, want true")
	}
}

func TestCircuitBreaker_HalfOpen_AllowsOneThenSuccess(t *testing.T) {
	b := NewCircuitBreaker(BreakerConfig{
		FailureThreshold: 1,
		Window:           time.Minute,
		ResetTimeout:     10 * time.Millisecond,
	})
	b.RecordFailure("A")
	if b.Allow("A") {
		t.Error("Allow(A) while open = true, want false")
	}
	time.Sleep(20 * time.Millisecond)
	if !b.Allow("A") {
		t.Error("Allow(A) after reset timeout = false, want true")
	}
	if b.Allow("A") {
		t.Error("Allow(A) second call in half-open = true, want false")
	}
	b.RecordSuccess("A")
	if !b.Allow("A") {
		t.Error("Allow(A) after success = false, want true")
	}
}

func TestCircuitBreaker_HalfOpen_FailureReopens(t *testing.T) {
	b := NewCircuitBreaker(BreakerConfig{
		FailureThreshold: 1,
		Window:           time.Minute,
		ResetTimeout:     10 * time.Millisecond,
	})
	b.RecordFailure("A")
	time.Sleep(20 * time.Millisecond)
	if !b.Allow("A") {
		t.Error("Allow(A) after reset = false, want true")
	}
	b.RecordFailure("A")
	if b.Allow("A") {
		t.Error("Allow(A) after failure in half-open = true, want false")
	}
}

func TestCircuitBreaker_SuccessClearsFailuresInClosed(t *testing.T) {
	b := NewCircuitBreaker(BreakerConfig{
		FailureThreshold: 2,
		Window:           time.Minute,
		ResetTimeout:     time.Second,
	})
	b.RecordFailure("A")
	b.RecordSuccess("A")
	b.RecordFailure("A")
	if !b.Allow("A") {
		t.Error("Allow(A) = false, want true (only 1 failure in window)")
	}
}

func TestBreakerMiddleware_RecordsOutcome(t *testing.T) {
	b := NewCircuitBreaker(BreakerConfig{
		FailureThreshold: 1,
		Window:           time.Minute,
		ResetTimeout:     time.Millisecond,
	})
	mw := BreakerMiddleware(b)
	h := mw(func(ctx context.Context, job *payload.Job) error { return nil })
	job := payload.NewJob("W", "q")
	err := h(nil, job)
	if err != nil {
		t.Errorf("handler = %v, want nil", err)
	}
	if !b.Allow("W") {
		t.Error("Allow(W) after success = false, want true")
	}

	h2 := mw(func(ctx context.Context, job *payload.Job) error { return errFake })
	err = h2(nil, job)
	if err != errFake {
		t.Errorf("handler = %v, want errFake", err)
	}
	if b.Allow("W") {
		t.Error("Allow(W) after failure = true, want false")
	}
}

var errFake = errors.New("fake")
