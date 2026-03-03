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
}

func NewProcessor(cfg *config.Config, b broker.Broker, registry WorkerRegistry) (*Processor, error) {
	return NewProcessorWithChain(cfg, b, registry, nil)
}

func NewProcessorWithChain(cfg *config.Config, b broker.Broker, registry WorkerRegistry, chain *Chain) (*Processor, error) {
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

	p := &Processor{
		cfg:         cfg,
		broker:      b,
		registry:    registry,
		log:         log,
		queues:      queues,
		jobCh:       make(chan jobMsg),
		ctx:         ctx,
		cancel:      cancel,
		retryTicker: time.NewTicker(5 * time.Second),
		chain:       chain,
	}

	return p, nil
}

func (p *Processor) Start() error {
	if p.chain != nil && p.handler == nil {
		base := func(ctx context.Context, job *payload.Job) error {
			if v := payload.GetDefaultValidator(); v != nil {
				if err := v.Validate(job); err != nil {
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

	for i := 0; i < p.cfg.Concurrency; i++ {
		p.wg.Add(1)
		go p.worker()
	}

	p.wg.Add(1)
	go p.retryProcessor()

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
			p.log.Debug("job fetched", "jid", job.JID, "queue", queue, "class", job.Class)

			select {
			case <-p.ctx.Done():
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
	ctx, cancel := context.WithTimeout(context.Background(), p.cfg.GetTimeout())
	defer cancel()

	handler := p.handler
	if handler == nil {
		handler = func(ctx context.Context, job *payload.Job) error {
			if v := payload.GetDefaultValidator(); v != nil {
				if err := v.Validate(job); err != nil {
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
		p.log.Error("job failed", "jid", job.JID, "err", err, "duration", duration)
		p.handleFailure(job, err)
	} else {
		p.log.Info("job succeeded", "jid", job.JID, "duration", duration)
	}
}

func (p *Processor) handleFailure(job *payload.Job, err error) {
	job.RetryCount++

	if job.RetryCount <= job.Retry {
		backoff := time.Duration(1<<uint(job.RetryCount)) * time.Second
		retryAt := time.Now().Add(backoff)
		p.log.Debug("scheduling retry", "jid", job.JID, "attempt", job.RetryCount, "max", job.Retry, "retryAt", retryAt)

		if err := p.broker.AddToRetry(job, retryAt); err != nil {
			p.log.Warn("add to retry failed", "jid", job.JID, "err", err)
		}
	} else {
		p.log.Warn("job exceeded max retries, moving to dead queue", "jid", job.JID)

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

		if err := p.broker.Enqueue(job.Queue, job); err != nil {
			p.log.Warn("re-enqueue retry failed", "jid", job.JID, "err", err)
			p.broker.AddToRetry(job, time.Now().Add(1*time.Minute))
		} else {
			p.log.Debug("re-enqueued retry job", "jid", job.JID, "queue", job.Queue)
		}
	}
}
