// Package sidekiq provides a Sidekiq-compatible job queue for Go.
// All public API is re-exported from internal packages.
package sidekiq

import (
	"regexp"
	"time"

	"github.com/quest/sidekiq-go/internal/broker"
	"github.com/quest/sidekiq-go/internal/config"
	"github.com/quest/sidekiq-go/internal/payload"
	"github.com/quest/sidekiq-go/internal/queue"
	"github.com/quest/sidekiq-go/pkg/sdk"
)

// ----- broker -----
type Broker = broker.Broker
type RedisClient = broker.RedisBroker
type RedisBrokerConfig = broker.RedisBrokerConfig

func NewRedisClient(url string, timeout time.Duration) (*RedisClient, error) {
	return broker.NewRedisBroker(url, timeout)
}

func NewRedisClientWithConfig(cfg RedisBrokerConfig) (*RedisClient, error) {
	return broker.NewRedisBrokerWithConfig(cfg)
}

// ----- payload / job -----
type Job = payload.Job
type JobOptions = payload.JobOptions

var (
	NewJob   = payload.NewJob
	FromJSON = payload.FromJSON
)

// ----- client (sdk) -----
type Client = sdk.Client

var (
	NewClient          = sdk.NewClient
	SetGlobalClient    = sdk.SetGlobalClient
	GetGlobalClient    = sdk.GetGlobalClient
	Enqueue            = sdk.EnqueueGlobal
	EnqueueWithOptions = sdk.EnqueueWithOptionsGlobal
)

// ----- config -----
type Config = config.Config
type QueueConfig = config.QueueConfig
type RedisConfig = config.RedisConfig

var LoadConfig = config.Load

// ----- queue / processor / worker / stats -----
type (
	Processor = queue.Processor
	Queue     = queue.Queue
	Stats     = queue.Stats
)

var (
	NewProcessor = queue.NewProcessor
	NewQueue     = queue.NewQueue
	GetStats     = queue.GetStats
)

// ----- worker -----
type Worker = queue.Worker

var (
	RegisterWorker = queue.RegisterWorker
	GetWorker      = queue.GetWorker
	ListWorkers    = queue.ListWorkers
)

// ----- middleware -----
type MiddlewareFunc = queue.MiddlewareFunc
type MiddlewareChain = queue.MiddlewareChain

var (
	NewMiddlewareChain  = queue.NewMiddlewareChain
	AddMiddleware       = queue.AddMiddleware
	GetMiddlewareChain  = queue.GetMiddlewareChain
	LoggingMiddleware   = queue.LoggingMiddleware
)

// ----- redactor -----
type Redactor = payload.Redactor

var (
	NoopRedactor    = payload.NoopRedactor{}
	MaskingRedactor = payload.MaskingRedactor{}
)

func SetRedactor(r payload.Redactor)   { payload.SetDefaultRedactor(r) }
func GetRedactor() payload.Redactor   { return payload.GetDefaultRedactor() }
func NewFieldMaskingRedactor(keys []string) *payload.FieldMaskingRedactor {
	return &payload.FieldMaskingRedactor{Keys: keys}
}

// ----- validator -----
type Validator = payload.Validator
type ChainValidator = payload.ChainValidator

var (
	MaxArgsCount   = payload.MaxArgsCount
	ClassAllowlist = payload.ClassAllowlist
	ClassPattern   = payload.ClassPattern
	MaxPayloadSize = payload.MaxPayloadSize
)

func SetValidator(v payload.Validator) { payload.SetDefaultValidator(v) }
func GetValidator() payload.Validator { return payload.GetDefaultValidator() }

func SafeClassPattern() payload.Validator {
	return payload.ClassPattern(regexp.MustCompile(`^[A-Za-z0-9_]+$`))
}
