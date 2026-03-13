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
	cfg         *config.Config
	broker      broker.Broker
	registry    WorkerRegistry
	log         Logger
	queues      []string
	jobCh       chan jobMsg
	wg          sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc
	retryTicker *time.Ticker
	chain       *Chain
	handler     Handler
	breaker     *CircuitBreaker

	metrics MetricsHandler
	events  chan JobEvent
}

func NewProcessor(cfg *config.Config, broker broker.Broker, registry WorkerRegistry, chain *Chain) (*Processor, error) {
	queues := make([]string, 0)
	for _, qc := range cfg.Queues {
		for i := 0; i < qc.Weight; i++ {
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
		chain = NewChain(
			RecoveryMiddleware(log),
			LoggingMiddleware(log),
		)
	}

	processor := &Processor{
		cfg:         cfg,
		broker:      broker,
		registry:    registry,
		log:         log,
		queues:      queues,
		jobCh:       make(chan jobMsg),
		ctx:         ctx,
		cancel:      cancel,
		retryTicker: time.NewTicker(5 * time.Second),
		chain:       chain,
	}

	return processor, nil
}

func (p *Processor) Start() error {
	if p.chain != nil && p.handler == nil {
		base := func(ctx context.Context, job *payload.Job) error {
			if validator := payload.GetDefaultValidator(); validator != nil {
				if err := validator.Validate(job); err != nil {
					return err
				}
			}

			var worker Worker
			var err error
			if p.registry != nil {
				worker, err = p.registry.GetWorker(job.Class)
			} else {
				worker, err = GetWorker(job.Class)
			}
			if err != nil {
				return fmt.Errorf("worker not found: %w", err)
			}

			return worker.Perform(ctx, job.Args...)
		}
		p.handler = p.chain.Wrap(base)
	}

	p.log.Info("engine started", "concurrency", p.cfg.Concurrency, "queues", p.queues)

	p.wg.Add(1)
	go p.fetcher()

	for workerSlot := 0; workerSlot < p.cfg.Concurrency; workerSlot++ {
		p.wg.Add(1)
		go p.worker()
	}

	p.wg.Add(1)
	go p.retryProcessor()

	if p.metrics != nil && p.events != nil {
		p.wg.Add(1)
		go p.metricsLoop()
	}

	return nil
}

func (p *Processor) Stop() {
	p.log.Info("engine stopping")
	p.cancel()
	p.retryTicker.Stop()
	p.wg.Wait()
	p.log.Info("engine stopped")
}

func (p *Processor) fetcher() {
	defer p.wg.Done()
	defer close(p.jobCh)

	dequeueTimeout := 1 * time.Second
	const openSleep = 5 * time.Second
	for {
		select {
		case <-p.ctx.Done():
			return
		default:
			job, queue, err := p.broker.Dequeue(p.queues, dequeueTimeout)
			if err != nil {
				p.log.Warn("dequeue failed", "err", err)
				continue
			}
			if job == nil {
				continue
			}
			if p.breaker != nil && !p.breaker.Allow(job.Class) {
				job.State = payload.JobStatePending
				if err := p.broker.Enqueue(queue, job); err != nil {
					p.log.Warn("requeue after breaker open failed", "jid", job.JID, "err", err)
				}
				select {
				case <-p.ctx.Done():
					return
				case <-time.After(openSleep):
				}
				continue
			}
			job.State = payload.JobStateActive
			p.log.Debug("job fetched", "jid", job.JID, "queue", queue, "class", job.Class, "state", job.State)

			select {
			case <-p.ctx.Done():
				job.State = payload.JobStatePending
				_ = p.broker.Enqueue(queue, job)
				return
			case p.jobCh <- jobMsg{job: job, queue: queue}:
			}
		}
	}
}

func (p *Processor) worker() {
	defer p.wg.Done()

	for {
		select {
		case <-p.ctx.Done():
			return
		case msg, ok := <-p.jobCh:
			if !ok {
				return
			}
			p.processJob(msg.job, msg.queue)
		}
	}
}

func (p *Processor) processJob(job *payload.Job, queue string) {
	start := time.Now()
	p.emitEvent(JobEvent{
		Type:  EventJobStarted,
		Job:   job,
		Queue: queue,
	})
	ctx, cancel := context.WithTimeout(context.Background(), p.cfg.GetTimeout())
	defer cancel()

	handler := p.handler
	if handler == nil {
		handler = func(ctx context.Context, job *payload.Job) error {
			if validator := payload.GetDefaultValidator(); validator != nil {
				if err := validator.Validate(job); err != nil {
					return err
				}
			}
			var worker Worker
			var err error
			if p.registry != nil {
				worker, err = p.registry.GetWorker(job.Class)
			} else {
				worker, err = GetWorker(job.Class)
			}
			if err != nil {
				return fmt.Errorf("worker not found: %w", err)
			}
			return worker.Perform(ctx, job.Args...)
		}
	}

	err := handler(ctx, job)
	duration := time.Since(start)

	if err != nil {
		job.State = payload.JobStateFailed
		p.log.Error("job failed", "jid", job.JID, "err", err, "duration", duration, "state", job.State)
		p.emitEvent(JobEvent{
			Type:     EventJobFailed,
			Job:      job,
			Queue:    queue,
			Duration: duration,
			Err:      err,
		})
		p.handleFailure(job, err)
	} else {
		job.State = payload.JobStateSuccess
		p.log.Info("job succeeded", "jid", job.JID, "duration", duration, "state", job.State)
		p.emitEvent(JobEvent{
			Type:     EventJobSucceeded,
			Job:      job,
			Queue:    queue,
			Duration: duration,
		})
	}
}

func (p *Processor) handleFailure(job *payload.Job, err error) {
	job.RetryCount++

	if job.RetryCount <= job.Retry {
		backoff := time.Duration(1<<uint(job.RetryCount)) * time.Second
		retryAt := time.Now().Add(backoff)
		p.log.Debug("scheduling retry", "jid", job.JID, "attempt", job.RetryCount, "max", job.Retry, "retryAt", retryAt, "state", job.State)

		p.emitEvent(JobEvent{
			Type:  EventJobRetryScheduled,
			Job:   job,
			Queue: job.Queue,
			Err:   err,
		})

		if err := p.broker.AddToRetry(job, retryAt); err != nil {
			p.log.Warn("add to retry failed", "jid", job.JID, "err", err)
		}
	} else {
		job.State = payload.JobStateDead
		p.log.Warn("job exceeded max retries, moving to dead queue", "jid", job.JID, "state", job.State)

		p.emitEvent(JobEvent{
			Type:  EventJobMovedToDead,
			Job:   job,
			Queue: job.Queue,
			Err:   err,
		})

		if err := p.broker.AddToDead(job); err != nil {
			p.log.Warn("add to dead failed", "jid", job.JID, "err", err)
		}
	}
}

func (p *Processor) retryProcessor() {
	defer p.wg.Done()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-p.retryTicker.C:
			p.processRetries()
		}
	}
}

func (p *Processor) processRetries() {
	jobs, err := p.broker.GetRetryJobs(100)
	if err != nil {
		p.log.Warn("get retry jobs failed", "err", err)
		return
	}

	for _, job := range jobs {
		if err := p.broker.RemoveFromRetry(job); err != nil {
			p.log.Warn("remove from retry failed", "jid", job.JID, "err", err)
			continue
		}

		job.State = payload.JobStatePending

		if err := p.broker.Enqueue(job.Queue, job); err != nil {
			p.log.Warn("re-enqueue retry failed", "jid", job.JID, "err", err)
			p.broker.AddToRetry(job, time.Now().Add(1*time.Minute))
		} else {
			p.log.Debug("re-enqueued retry job", "jid", job.JID, "queue", job.Queue, "state", job.State)
		}
	}
}

func (p *Processor) emitEvent(event JobEvent) {
	if p.metrics == nil || p.events == nil {
		return
	}

	select {
	case p.events <- event:
	default:
		p.log.Debug("metrics event dropped", "type", event.Type, "jid", event.Job.JID)
	}
}

func (p *Processor) metricsLoop() {
	defer p.wg.Done()

	for {
		select {
		case <-p.ctx.Done():
			return
		case event, ok := <-p.events:
			if !ok {
				return
			}

			func() {
				defer func() {
					if panicValue := recover(); panicValue != nil {
						p.log.Warn("panic in metrics handler", "panic", panicValue)
					}
				}()

				p.metrics.HandleJobEvent(context.Background(), event)
			}()
		}
	}
}

// SetCircuitBreaker sets the circuit breaker used by the fetcher to skip open job classes.
func (p *Processor) SetCircuitBreaker(b *CircuitBreaker) {
	p.breaker = b
}

func (p *Processor) SetMetricsHandler(handler MetricsHandler) {
	p.metrics = handler
	if handler == nil {
		return
	}
	if p.events != nil {
		return
	}

	size := p.cfg.Concurrency * 10
	if size <= 0 {
		size = 10
	}
	p.events = make(chan JobEvent, size)
}
