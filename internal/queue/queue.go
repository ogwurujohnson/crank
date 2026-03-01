package queue

import (
	"github.com/quest/sidekiq-go/internal/broker"
)

// Queue represents a job queue
type Queue struct {
	name   string
	broker broker.Broker
}

// NewQueue creates a new queue instance
func NewQueue(name string, b broker.Broker) *Queue {
	return &Queue{
		name:   name,
		broker: b,
	}
}

// Size returns the current size of the queue
func (q *Queue) Size() (int64, error) {
	return q.broker.GetQueueSize(q.name)
}

// Name returns the queue name
func (q *Queue) Name() string {
	return q.name
}

// Clear clears all jobs from the queue
func (q *Queue) Clear() error {
	return q.broker.DeleteKey("queue:" + q.name)
}

// Stats provides queue statistics
type Stats struct {
	Processed int64            `json:"processed"`
	Retry     int64            `json:"retry"`
	Dead      int64            `json:"dead"`
	Queues    map[string]int64 `json:"queues"`
}

// GetStats returns current statistics
func GetStats(b broker.Broker) (*Stats, error) {
	statsMap, err := b.GetStats()
	if err != nil {
		return nil, err
	}

	stats := &Stats{
		Processed: statsMap["processed"].(int64),
		Retry:     statsMap["retry"].(int64),
		Dead:      statsMap["dead"].(int64),
		Queues:    statsMap["queues"].(map[string]int64),
	}

	return stats, nil
}
