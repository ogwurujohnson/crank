package crank

import (
	"fmt"
	"sync"

	"github.com/ogwurujohnson/crank/internal/broker"
	"github.com/ogwurujohnson/crank/internal/config"
	"github.com/ogwurujohnson/crank/internal/queue"
)

type engineRegistry struct {
	mu      sync.RWMutex
	workers map[string]queue.Worker
}

func (r *engineRegistry) GetWorker(className string) (queue.Worker, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	worker, ok := r.workers[className]
	if !ok {
		return nil, fmt.Errorf("worker class '%s' not found", className)
	}
	return worker, nil
}

func (r *engineRegistry) register(className string, worker queue.Worker) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.workers == nil {
		r.workers = make(map[string]queue.Worker)
	}
	r.workers[className] = worker
}

type Engine struct {
	cfg       *config.Config
	broker    broker.Broker
	processor *queue.Processor
	registry  *engineRegistry
	chain     *queue.Chain
}

func NewEngine(cfg *Config, broker Broker) (*Engine, error) {
	if cfg.Logger == nil {
		cfg.Logger = queue.NopLogger()
	}
	
	registry := &engineRegistry{workers: make(map[string]queue.Worker)}
	breaker := queue.NewCircuitBreaker(queue.BreakerConfig{})
	chain := queue.NewChain(
		queue.RecoveryMiddleware(cfg.Logger),
		queue.LoggingMiddleware(cfg.Logger),
		queue.BreakerMiddleware(breaker),
	)
	processor, err := queue.NewProcessor(cfg, broker, registry, chain)
	if err != nil {
		return nil, err
	}

	return &Engine{
		cfg:       cfg,
		broker:    broker,
		processor: processor,
		registry:  registry,
		chain:     chain,
	}, nil
}

func (e *Engine) Use(middleware ...Middleware) {
	if e.chain == nil {
		return
	}
	e.chain.Use(middleware...)
}

func (e *Engine) Register(className string, worker Worker) {
	e.registry.register(className, worker)
}

func (e *Engine) RegisterMany(workers map[string]Worker) {
	for name, worker := range workers {
		e.registry.register(name, worker)
	}
}

func (e *Engine) Start() error {
	return e.processor.Start()
}

func (e *Engine) Stop() {
	e.processor.Stop()
}
