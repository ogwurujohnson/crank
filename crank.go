// Package crank is a background job queue for Go.
package crank

import (
	"regexp"
	"time"

	"github.com/ogwurujohnson/crank/internal/broker"
	"github.com/ogwurujohnson/crank/internal/config"
	"github.com/ogwurujohnson/crank/internal/payload"
	"github.com/ogwurujohnson/crank/internal/queue"
	"github.com/ogwurujohnson/crank/pkg/sdk"
)

type Broker = broker.Broker
type RedisClient = broker.RedisBroker
type RedisBrokerConfig = broker.RedisBrokerConfig

func NewRedisClient(url string, timeout time.Duration) (*RedisClient, error) {
	return broker.NewRedisBroker(url, timeout)
}

func NewRedisClientWithConfig(cfg RedisBrokerConfig) (*RedisClient, error) {
	return broker.NewRedisBrokerWithConfig(cfg)
}

type Job = payload.Job
type JobOptions = payload.JobOptions

var (
	NewJob   = payload.NewJob
	FromJSON = payload.FromJSON
)

type Client = sdk.Client

var (
	NewClient          = sdk.NewClient
	SetGlobalClient    = sdk.SetGlobalClient
	GetGlobalClient    = sdk.GetGlobalClient
	Enqueue            = sdk.EnqueueGlobal
	EnqueueWithOptions = sdk.EnqueueWithOptionsGlobal
)

type Config = config.Config
type QueueConfig = config.QueueConfig
type RedisConfig = config.RedisConfig
type Logger = config.Logger

var LoadConfig = config.Load

type (
	Processor      = queue.Processor
	Queue          = queue.Queue
	Stats          = queue.Stats
	MetricsHandler = queue.MetricsHandler
	JobEvent       = queue.JobEvent
	EventType      = queue.EventType
)

var (
	NewQueue  = queue.NewQueue
	GetStats  = queue.GetStats
	NopLogger = queue.NopLogger
)

const (
	EventJobStarted        = queue.EventJobStarted
	EventJobSucceeded      = queue.EventJobSucceeded
	EventJobFailed         = queue.EventJobFailed
	EventJobRetryScheduled = queue.EventJobRetryScheduled
	EventJobMovedToDead    = queue.EventJobMovedToDead
)

type Worker = queue.Worker

var (
	RegisterWorker = queue.RegisterWorker
	GetWorker      = queue.GetWorker
	ListWorkers    = queue.ListWorkers
)

type Handler = queue.Handler
type Middleware = queue.Middleware
type Chain = queue.Chain

var (
	NewChain           = queue.NewChain
	LoggingMiddleware  = queue.LoggingMiddleware
	RecoveryMiddleware = queue.RecoveryMiddleware
)

type Redactor = payload.Redactor

var (
	NoopRedactor    = payload.NoopRedactor{}
	MaskingRedactor = payload.MaskingRedactor{}
)

func SetRedactor(r payload.Redactor) { payload.SetDefaultRedactor(r) }
func GetRedactor() payload.Redactor  { return payload.GetDefaultRedactor() }
func NewFieldMaskingRedactor(keys []string) *payload.FieldMaskingRedactor {
	return &payload.FieldMaskingRedactor{Keys: keys}
}

type Validator = payload.Validator
type ChainValidator = payload.ChainValidator

var (
	MaxArgsCount   = payload.MaxArgsCount
	ClassAllowlist = payload.ClassAllowlist
	ClassPattern   = payload.ClassPattern
	MaxPayloadSize = payload.MaxPayloadSize
)

func SetValidator(v payload.Validator) { payload.SetDefaultValidator(v) }
func GetValidator() payload.Validator  { return payload.GetDefaultValidator() }

func SafeClassPattern() payload.Validator {
	return payload.ClassPattern(regexp.MustCompile(`^[A-Za-z0-9_]+$`))
}
