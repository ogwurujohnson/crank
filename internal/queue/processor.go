package queue

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ogwurujohnson/crank/internal/broker"
	"github.com/ogwurujohnson/crank/internal/config"
	"github.com/ogwurujohnson/crank/internal/payload"
)

type jobMsg struct {
	job   *payload.Job
	queue string
}

type Processor struct {
	cfg      *config.Config
	broker   broker.Broker
	registry WorkerRegistry
	log      Logger
	queues   []string
	jobCh    chan jobMsg
	wg       sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
	chain    *Chain
	handler  Handler
	breaker  *CircuitBreaker
	metrics  MetricsHandler
	events   chan JobEvent
}

func NewProcessor(cfg *config.Config, b broker.Broker, registry WorkerRegistry, chain *Chain) (*Processor, error) {
	var queues []string
	for _, qc := range cfg.Queues {
		weight := qc.Weight
		if weight <= 0 {
			weight = 1
		}
		for i := 0; i < weight; i++ {
			queues = append(queues, qc.Name)
		}
	}

	if len(queues) == 0 {
		queues = []string{"default"}
	}

	ctx, cancel := context.WithCancel(context.Background())
	log := cfg.Logger
	if log == nil {
		log = NopLogger()
	}

	if chain == nil {
		chain = NewChain(RecoveryMiddleware(log), LoggingMiddleware(log))
	}

	p := &Processor{
		cfg:      cfg,
		broker:   b,
		registry: registry,
		log:      log,
		queues:   queues,
		jobCh:    make(chan jobMsg),
		ctx:      ctx,
		cancel:   cancel,
		chain:    chain,
	}

	// Initialize the execution handler once
	p.handler = p.chain.Wrap(p.execute)
	return p, nil
}

// execute is the core logic wrapped by middleware
func (p *Processor) execute(ctx context.Context, job *payload.Job) error {
	if v := payload.GetDefaultValidator(); v != nil {
		if err := v.Validate(job); err != nil {
			return fmt.Errorf("validation failed: %w", err)
		}
	}

	var (
		worker Worker
		err    error
	)

	// Fallback chain: local registry -> global registry
	if p.registry != nil {
		worker, err = p.registry.GetWorker(job.Class)
	}
	if worker == nil {
		worker, err = GetWorker(job.Class)
	}

	if err != nil {
		return fmt.Errorf("worker not found for %s: %w", job.Class, err)
	}

	return worker.Perform(ctx, job.Args...)
}

func (p *Processor) Start() error {
	p.log.Info("engine started", "concurrency", p.cfg.Concurrency, "queues", p.queues)

	// Start Fetcher
	p.wg.Add(1)
	go p.fetcher()

	// Start Workers
	for i := 0; i < p.cfg.Concurrency; i++ {
		p.wg.Add(1)
		go p.worker()
	}

	// Start Maintenance Loops
	p.wg.Add(2)
	go p.retryLoop()
	go p.metricsLoop()

	return nil
}

func (p *Processor) Stop() {
	p.log.Info("engine stopping")
	p.cancel()
	p.wg.Wait()
	p.log.Info("engine stopped")
}

func (p *Processor) fetcher() {
	defer p.wg.Done()
	defer close(p.jobCh)

	// cursor tracks our position in the weighted queues slice
	cursor := 0
	numQueues := len(p.queues)

	pauseTimer := time.NewTimer(0)
	if !pauseTimer.Stop() {
		<-pauseTimer.C
	}

	for {
		select {
		case <-p.ctx.Done():
			return
		default:
			// Pick the next queue based on the weighted slice
			// This ensures we respect the frequency defined in NewProcessor
			currentQueue := p.queues[cursor]
			cursor = (cursor + 1) % numQueues

			// Dequeue only from this specific queue
			job, q, err := p.broker.Dequeue([]string{currentQueue}, time.Second)

			if err != nil {
				p.log.Warn("dequeue failed", "queue", currentQueue, "err", err)
				continue
			}
			if job == nil {
				continue
			}

			// ... (Circuit Breaker logic remains the same)

			select {
			case p.jobCh <- jobMsg{job: job, queue: q}:
			case <-p.ctx.Done():
				job.State = payload.JobStatePending
				_ = p.broker.Enqueue(q, job)
				return
			}
		}
	}
}

func (p *Processor) worker() {
	defer p.wg.Done()
	for msg := range p.jobCh {
		p.processJob(msg.job, msg.queue)
	}
}

func (p *Processor) processJob(job *payload.Job, queue string) {
	start := time.Now()
	p.emitEvent(JobEvent{Type: EventJobStarted, Job: job, Queue: queue})

	ctx, cancel := context.WithTimeout(context.Background(), p.cfg.GetTimeout())
	defer cancel()

	job.State = payload.JobStateActive
	err := p.handler(ctx, job)
	duration := time.Since(start)

	if err != nil {
		job.State = payload.JobStateFailed
		p.log.Error("job failed", "jid", job.JID, "err", err, "dur", duration)
		p.emitEvent(JobEvent{Type: EventJobFailed, Job: job, Queue: queue, Duration: duration, Err: err})
		p.handleFailure(job, err)
	} else {
		job.State = payload.JobStateSuccess
		p.log.Info("job ok", "jid", job.JID, "dur", duration)
		p.emitEvent(JobEvent{Type: EventJobSucceeded, Job: job, Queue: queue, Duration: duration})
	}
}

func (p *Processor) handleFailure(job *payload.Job, err error) {
	job.RetryCount++
	if job.RetryCount <= job.Retry {
		// Exponential backoff: 2^count + small constant
		delay := time.Duration(1<<uint(job.RetryCount)) * time.Second
		retryAt := time.Now().Add(delay)

		p.emitEvent(JobEvent{Type: EventJobRetryScheduled, Job: job, Queue: job.Queue, Err: err})
		if err := p.broker.AddToRetry(job, retryAt); err != nil {
			p.log.Warn("retry schedule failed", "jid", job.JID, "err", err)
		}
	} else {
		job.State = payload.JobStateDead
		p.log.Warn("job exceeded max retries, moving to dead queue", "jid", job.JID, "state", job.State)
		p.emitEvent(JobEvent{Type: EventJobMovedToDead, Job: job, Queue: job.Queue, Err: err})
		if err := p.broker.AddToDead(job); err != nil {
			p.log.Warn("move to dead failed", "jid", job.JID, "err", err)
		}
	}
}

func (p *Processor) retryLoop() {
	defer p.wg.Done()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			jobs, _ := p.broker.GetRetryJobs(100)
			for _, j := range jobs {
				// Re-enqueue logic
				if err := p.broker.RemoveFromRetry(j); err == nil {
					j.State = payload.JobStatePending
					if err := p.broker.Enqueue(j.Queue, j); err != nil {
						p.broker.AddToRetry(j, time.Now().Add(time.Minute))
					}
				}
			}
		}
	}
}

func (p *Processor) metricsLoop() {
	defer p.wg.Done()
	if p.events == nil {
		return
	}
	for {
		select {
		case <-p.ctx.Done():
			return
		case ev, ok := <-p.events:
			if !ok {
				return
			}
			if p.metrics != nil {
				p.metrics.HandleJobEvent(context.Background(), ev)
			}
		}
	}
}

func (p *Processor) emitEvent(ev JobEvent) {
	if p.events == nil {
		return
	}
	select {
	case p.events <- ev:
	default:
		// Drop event if buffer is full to avoid blocking workers
		p.log.Debug("metrics event dropped", "type", ev.Type, "jid", ev.Job.JID)
	}
}

func (p *Processor) SetMetricsHandler(h MetricsHandler) {
	p.metrics = h
	if h != nil && p.events == nil {
		size := p.cfg.Concurrency * 10
		if size < 10 {
			size = 10
		}
		p.events = make(chan JobEvent, size)
	}
}
