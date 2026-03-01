package broker

import (
	"time"

	"github.com/ogwurujohnson/crank/internal/payload"
)

// Broker defines the storage abstraction for job queues.
// Implementations can use Redis, PostgreSQL, NATS, or other backends.
type Broker interface {
	Enqueue(queue string, job *payload.Job) error
	Dequeue(queues []string, timeout time.Duration) (*payload.Job, string, error)
	Ack(job *payload.Job) error
	AddToRetry(job *payload.Job, retryAt time.Time) error
	GetRetryJobs(limit int64) ([]*payload.Job, error)
	RemoveFromRetry(job *payload.Job) error
	AddToDead(job *payload.Job) error
	GetQueueSize(queue string) (int64, error)
	DeleteKey(key string) error
	GetStats() (map[string]interface{}, error)
	Close() error
}
