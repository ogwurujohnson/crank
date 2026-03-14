package broker

import (
	"time"

	"github.com/ogwurujohnson/crank/internal/payload"
)

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
	Close() error
}

// ConnOptions holds connection options used when opening a broker via Open.
// Backends map these to their own config (e.g. Redis timeout/TLS).
type ConnOptions struct {
	Timeout               time.Duration
	UseTLS                bool
	TLSInsecureSkipVerify bool
}
