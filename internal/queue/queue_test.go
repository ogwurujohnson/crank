package queue

import (
	"errors"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/ogwurujohnson/crank/internal/broker"
	"github.com/ogwurujohnson/crank/internal/payload"
)

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

func (m *mockBroker) Enqueue(string, *payload.Job) error { return nil }
func (m *mockBroker) Dequeue([]string, time.Duration) (*payload.Job, string, error) {
	return nil, "", nil
}
func (m *mockBroker) AddToRetry(*payload.Job, time.Time) error   { return nil }
func (m *mockBroker) GetRetryJobs(int64) ([]*payload.Job, error) { return nil, nil }
func (m *mockBroker) RemoveFromRetry(*payload.Job) error         { return nil }
func (m *mockBroker) AddToDead(*payload.Job) error               { return nil }
func (m *mockBroker) GetDeadJobs(int64) ([]*payload.Job, error)  { return nil, nil }
func (m *mockBroker) GetQueueSize(string) (int64, error)         { return 0, nil }
func (m *mockBroker) DeleteKey(string) error                     { return nil }
func (m *mockBroker) Close() error                               { return nil }

var _ broker.Broker = (*mockBroker)(nil)

func TestGetStats_Valid(t *testing.T) {
	c := qt.New(t)
	b := &mockBroker{
		stats: map[string]interface{}{
			"processed": int64(10),
			"retry":     int64(1),
			"dead":      int64(0),
			"queues":    map[string]int64{"default": 5, "low": 2},
		},
	}
	stats, err := GetStats(b)
	c.Assert(err, qt.IsNil)
	c.Assert(stats, qt.DeepEquals, &Stats{
		Processed: 10,
		Retry:     1,
		Dead:      0,
		Queues:    map[string]int64{"default": 5, "low": 2},
	})
}

func TestGetStats_IntAndFloat64(t *testing.T) {
	c := qt.New(t)
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
	c.Assert(err, qt.IsNil)
	c.Assert(stats, qt.DeepEquals, &Stats{
		Processed: 100,
		Retry:     2,
		Dead:      1,
		Queues:    map[string]int64{"default": 3, "low": 1},
	})
}

func TestGetStats_BrokerError(t *testing.T) {
	c := qt.New(t)
	b := &mockBroker{err: errors.New("broker down")}
	_, err := GetStats(b)
	c.Assert(err, qt.ErrorMatches, "broker down")
}

func TestGetStats_MissingKey(t *testing.T) {
	c := qt.New(t)
	b := &mockBroker{
		stats: map[string]interface{}{
			"processed": int64(1),
		},
	}
	_, err := GetStats(b)
	c.Assert(err, qt.ErrorMatches, `stats: missing or nil "retry"`)
}

func TestGetStats_InvalidType(t *testing.T) {
	c := qt.New(t)
	b := &mockBroker{
		stats: map[string]interface{}{
			"processed": int64(1),
			"retry":     int64(1),
			"dead":      int64(1),
			"queues":    "not a map",
		},
	}
	_, err := GetStats(b)
	c.Assert(err, qt.ErrorMatches, `stats: "queues" is not a map`)
}

func TestGetStats_NilValue(t *testing.T) {
	c := qt.New(t)
	b := &mockBroker{
		stats: map[string]interface{}{
			"processed": nil,
			"retry":     int64(0),
			"dead":      int64(0),
			"queues":    map[string]int64{},
		},
	}
	_, err := GetStats(b)
	c.Assert(err, qt.ErrorMatches, `stats: missing or nil "processed"`)
}
