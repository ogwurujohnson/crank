package broker

import (
	"time"

	"github.com/ogwurujohnson/crank/internal/payload"
)

// RetrySchedule is a job waiting in the retry sorted set with its scheduled run time.
type RetrySchedule struct {
	Job     *payload.Job `json:"job"`
	RetryAt time.Time    `json:"retry_at"`
}

// Broker is the internal backend-agnostic interface for job queues.
// Implementations (Redis, NATS, RabbitMQ, etc.) live in this package and are
// selected by Open(url, opts) based on the URL scheme. Callers outside this
// package never reference a concrete backend—they use Broker only.
type Broker interface {
	Enqueue(queue string, job *payload.Job) error
	Dequeue(queues []string, timeout time.Duration) (*payload.Job, string, error)
	AddToRetry(job *payload.Job, retryAt time.Time) error
	GetRetryJobs(limit int64) ([]*payload.Job, error)
	RemoveFromRetry(job *payload.Job) error
	AddToDead(job *payload.Job) error
	GetDeadJobs(limit int64) ([]*payload.Job, error)
	GetQueueSize(queue string) (int64, error)
	DeleteKey(key string) error
	GetStats() (map[string]interface{}, error)
	// PeekQueue returns up to limit jobs from the named queue without removing them
	// (oldest / next-to-dequeue first). Limit is clamped by each implementation.
	PeekQueue(queue string, limit int64) ([]*payload.Job, error)
	// ListRetryScheduled returns jobs in the retry set with their scheduled times,
	// ordered soonest first.
	ListRetryScheduled(limit int64) ([]RetrySchedule, error)
	Close() error
}

// ConnOptions holds connection options used when opening a broker via Open.
// Backends map these to their own config (e.g. Redis timeout/TLS).
type ConnOptions struct {
	Timeout               time.Duration
	UseTLS                bool
	TLSInsecureSkipVerify bool
}
