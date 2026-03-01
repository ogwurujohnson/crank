package sidekiq

import (
	"fmt"
)

// Queue represents a job queue
type Queue struct {
	name  string
	redis *RedisClient
}

// NewQueue creates a new queue instance
func NewQueue(name string, redis *RedisClient) *Queue {
	return &Queue{
		name:  name,
		redis: redis,
	}
}

// Size returns the current size of the queue
func (q *Queue) Size() (int64, error) {
	return q.redis.GetQueueSize(q.name)
}

// Name returns the queue name
func (q *Queue) Name() string {
	return q.name
}

// Clear clears all jobs from the queue
func (q *Queue) Clear() error {
	queueKey := fmt.Sprintf("queue:%s", q.name)
	return q.redis.DeleteKey(queueKey)
}

// Stats provides queue statistics
type Stats struct {
	Processed int64            `json:"processed"`
	Retry     int64            `json:"retry"`
	Dead      int64            `json:"dead"`
	Queues    map[string]int64 `json:"queues"`
}

// GetStats returns current statistics
func GetStats(redis *RedisClient) (*Stats, error) {
	statsMap, err := redis.GetStats()
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
