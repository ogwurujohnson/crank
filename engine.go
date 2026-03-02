package crank

import (
	"fmt"
	"sync"

	"github.com/ogwurujohnson/crank/internal/broker"
	"github.com/ogwurujohnson/crank/internal/config"
	"github.com/ogwurujohnson/crank/internal/queue"
)

// engineRegistry is the per-engine worker registry used by Processor.
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

// Engine runs the job processor and owns its worker registry.
// Create with NewEngine, register workers, then Start/Stop.
type Engine struct {
	cfg       *config.Config
	broker    broker.Broker
	processor *queue.Processor
	registry  *engineRegistry
}

// NewEngine creates an engine with the given config and broker.
// Register workers with engine.Register before calling Start.
func NewEngine(cfg *Config, broker Broker) (*Engine, error) {
	if cfg.Logger == nil {
		cfg.Logger = queue.NopLogger()
	}
	registry := &engineRegistry{workers: make(map[string]queue.Worker)}
	processor, err := queue.NewProcessor(cfg, broker, registry)
	if err != nil {
		return nil, err
	}

	return &Engine{
		cfg:       cfg,
		broker:    broker,
		processor: processor,
		registry:  registry,
	}, nil
}

// Register adds a worker for the given class name (e.g. "EmailWorker").
// Call before Start.
func (e *Engine) Register(className string, worker Worker) {
	e.registry.register(className, worker)
}

// RegisterWorkers adds multiple workers in one call. Keys are class names.
// Call before Start.
func (e *Engine) RegisterWorkers(workers map[string]Worker) {
	for name, w := range workers {
		e.registry.register(name, w)
	}
}

// Start begins processing jobs. Returns immediately; run Stop() to shut down.
func (e *Engine) Start() error {
	return e.processor.Start()
}

// Stop gracefully stops the engine (stops accepting new work, waits for in-flight jobs).
func (e *Engine) Stop() {
	e.processor.Stop()
}
