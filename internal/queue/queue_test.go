package queue

import (
	"errors"
	"testing"
	"time"

	"github.com/ogwurujohnson/crank/internal/broker"
	"github.com/ogwurujohnson/crank/internal/payload"
)

// mockBroker only implements GetStats for queue.GetStats tests.
type mockBroker struct {
	stats map[string]interface{}
	err   error
}

func (m *mockBroker) GetStats() (map[string]interface{}, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.stats, nil
}

func (m *mockBroker) Enqueue(string, *payload.Job) error              { return nil }
func (m *mockBroker) Dequeue([]string, time.Duration) (*payload.Job, string, error) {
	return nil, "", nil
}
func (m *mockBroker) Ack(*payload.Job) error                            { return nil }
func (m *mockBroker) AddToRetry(*payload.Job, time.Time) error           { return nil }
func (m *mockBroker) GetRetryJobs(int64) ([]*payload.Job, error)        { return nil, nil }
func (m *mockBroker) RemoveFromRetry(*payload.Job) error                 { return nil }
func (m *mockBroker) AddToDead(*payload.Job) error                       { return nil }
func (m *mockBroker) GetDeadJobs(int64) ([]*payload.Job, error)         { return nil, nil }
func (m *mockBroker) GetQueueSize(string) (int64, error)                { return 0, nil }
func (m *mockBroker) DeleteKey(string) error                            { return nil }
func (m *mockBroker) Close() error                                      { return nil }

var _ broker.Broker = (*mockBroker)(nil)

func TestGetStats_Valid(t *testing.T) {
	b := &mockBroker{
		stats: map[string]interface{}{
			"processed": int64(10),
			"retry":     int64(1),
			"dead":      int64(0),
			"queues":    map[string]int64{"default": 5, "low": 2},
		},
	}
	stats, err := GetStats(b)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Processed != 10 || stats.Retry != 1 || stats.Dead != 0 {
		t.Errorf("got %+v", stats)
	}
	if stats.Queues["default"] != 5 || stats.Queues["low"] != 2 {
		t.Errorf("queues: %v", stats.Queues)
	}
}

func TestGetStats_IntAndFloat64(t *testing.T) {
	// Simulate JSON unmarshaling (numbers as float64) and int
	b := &mockBroker{
		stats: map[string]interface{}{
			"processed": float64(100),
			"retry":     int(2),
			"dead":      float64(1),
			"queues": map[string]interface{}{
				"default": float64(3),
				"low":     int(1),
			},
		},
	}
	stats, err := GetStats(b)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Processed != 100 || stats.Retry != 2 || stats.Dead != 1 {
		t.Errorf("got %+v", stats)
	}
	if stats.Queues["default"] != 3 || stats.Queues["low"] != 1 {
		t.Errorf("queues: %v", stats.Queues)
	}
}

func TestGetStats_BrokerError(t *testing.T) {
	b := &mockBroker{err: errors.New("broker down")}
	_, err := GetStats(b)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "broker down" {
		t.Errorf("got %v", err)
	}
}

func TestGetStats_MissingKey(t *testing.T) {
	b := &mockBroker{
		stats: map[string]interface{}{
			"processed": int64(1),
			// missing "retry", "dead", "queues"
		},
	}
	_, err := GetStats(b)
	if err == nil {
		t.Fatal("expected error for missing key")
	}
}

func TestGetStats_InvalidType(t *testing.T) {
	b := &mockBroker{
		stats: map[string]interface{}{
			"processed": int64(1),
			"retry":     int64(1),
			"dead":      int64(1),
			"queues":    "not a map",
		},
	}
	_, err := GetStats(b)
	if err == nil {
		t.Fatal("expected error for invalid queues type")
	}
}

func TestGetStats_NilValue(t *testing.T) {
	b := &mockBroker{
		stats: map[string]interface{}{
			"processed": nil,
			"retry":     int64(0),
			"dead":      int64(0),
			"queues":    map[string]int64{},
		},
	}
	_, err := GetStats(b)
	if err == nil {
		t.Fatal("expected error for nil processed")
	}
}
