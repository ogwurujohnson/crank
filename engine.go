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
	w, ok := r.workers[className]
	if !ok {
		return nil, fmt.Errorf("worker class '%s' not found", className)
	}
	return w, nil
}

func (r *engineRegistry) register(className string, w queue.Worker) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.workers == nil {
		r.workers = make(map[string]queue.Worker)
	}
	r.workers[className] = w
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
	chain := queue.NewChain(
		queue.RecoveryMiddleware(cfg.Logger),
		queue.LoggingMiddleware(cfg.Logger),
	)
	processor, err := queue.NewProcessorWithChain(cfg, broker, registry, chain)
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

func (e *Engine) Use(m Middleware) {
	if e.chain == nil {
		return
	}
	e.chain.Use(m)
}

func (e *Engine) Register(className string, worker Worker) {
	e.registry.register(className, worker)
}

func (e *Engine) RegisterMany(workers map[string]Worker) {
	for name, w := range workers {
		e.registry.register(name, w)
	}
}

func (e *Engine) Start() error {
	return e.processor.Start()
}

func (e *Engine) Stop() {
	e.processor.Stop()
}

func (e *Engine) SetMetricsHandler(h MetricsHandler) {
	if e.processor == nil {
		return
	}
	e.processor.SetMetricsHandler(h)
}
