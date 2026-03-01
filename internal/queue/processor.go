package queue

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/quest/sidekiq-go/internal/broker"
	"github.com/quest/sidekiq-go/internal/config"
	"github.com/quest/sidekiq-go/internal/payload"
)

// Processor manages job processing with a worker pool
type Processor struct {
	cfg         *config.Config
	broker      broker.Broker
	queues      []string
	workers     chan struct{}
	wg          sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc
	retryTicker *time.Ticker
	verbose     bool
}

// NewProcessor creates a new processor instance
func NewProcessor(cfg *config.Config, b broker.Broker) (*Processor, error) {
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

	p := &Processor{
		cfg:         cfg,
		broker:      b,
		queues:      queues,
		workers:     make(chan struct{}, cfg.Concurrency),
		ctx:         ctx,
		cancel:      cancel,
		retryTicker: time.NewTicker(5 * time.Second),
		verbose:     cfg.Verbose,
	}

	return p, nil
}

// Start begins processing jobs
func (p *Processor) Start() error {
	if p.verbose {
		log.Printf("Starting Sidekiq processor with concurrency=%d, queues=%v", p.cfg.Concurrency, p.queues)
	}

	for i := 0; i < p.cfg.Concurrency; i++ {
		p.wg.Add(1)
		go p.worker()
	}

	p.wg.Add(1)
	go p.retryProcessor()

	return nil
}

// Stop gracefully stops the processor
func (p *Processor) Stop() {
	if p.verbose {
		log.Println("Stopping Sidekiq processor...")
	}

	p.cancel()
	p.retryTicker.Stop()
	p.wg.Wait()

	if p.verbose {
		log.Println("Sidekiq processor stopped")
	}
}

func (p *Processor) worker() {
	defer p.wg.Done()

	for {
		select {
		case <-p.ctx.Done():
			return
		default:
			p.workers <- struct{}{}

			job, queue, err := p.broker.Dequeue(p.queues, 1*time.Second)
			if err != nil {
				<-p.workers
				if p.verbose {
					log.Printf("Error dequeuing job: %v", err)
				}
				continue
			}

			if job == nil {
				<-p.workers
				continue
			}

			go func(j *payload.Job, q string) {
				defer func() { <-p.workers }()
				p.processJob(j, q)
			}(job, queue)
		}
	}
}

func (p *Processor) processJob(job *payload.Job, queue string) {
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), p.cfg.GetTimeout())
	defer cancel()

	if p.verbose {
		log.Printf("Processing %s", job)
	}

	if v := payload.GetDefaultValidator(); v != nil {
		if err := v.Validate(job); err != nil {
			log.Printf("Job %s failed validation: %v", job.JID, err)
			p.handleFailure(job, err)
			return
		}
	}

	worker, err := GetWorker(job.Class)
	if err != nil {
		log.Printf("Error getting worker '%s': %v", job.Class, err)
		p.handleFailure(job, fmt.Errorf("worker not found: %w", err))
		return
	}

	chain := GetMiddlewareChain()
	err = chain.Execute(ctx, job, func() error {
		return worker.Perform(ctx, job.Args...)
	})
	duration := time.Since(start)

	if err != nil {
		if p.verbose {
			log.Printf("Job %s failed: %v (took %v)", job.JID, err, duration)
		}
		p.handleFailure(job, err)
	} else {
		if p.verbose {
			log.Printf("Job %s completed successfully (took %v)", job.JID, duration)
		}
	}
}

func (p *Processor) handleFailure(job *payload.Job, err error) {
	job.RetryCount++

	if job.RetryCount <= job.Retry {
		backoff := time.Duration(1<<uint(job.RetryCount)) * time.Second
		retryAt := time.Now().Add(backoff)

		if p.verbose {
			log.Printf("Scheduling retry for job %s (attempt %d/%d) at %v", job.JID, job.RetryCount, job.Retry, retryAt)
		}

		if err := p.broker.AddToRetry(job, retryAt); err != nil {
			log.Printf("Error adding job to retry: %v", err)
		}
	} else {
		if p.verbose {
			log.Printf("Job %s exceeded max retries, moving to dead queue", job.JID)
		}

		if err := p.broker.AddToDead(job); err != nil {
			log.Printf("Error adding job to dead queue: %v", err)
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
		if p.verbose {
			log.Printf("Error getting retry jobs: %v", err)
		}
		return
	}

	for _, job := range jobs {
		if err := p.broker.RemoveFromRetry(job); err != nil {
			if p.verbose {
				log.Printf("Error removing job from retry: %v", err)
			}
			continue
		}

		if err := p.broker.Enqueue(job.Queue, job); err != nil {
			if p.verbose {
				log.Printf("Error re-enqueueing retry job: %v", err)
			}
			p.broker.AddToRetry(job, time.Now().Add(1*time.Minute))
		} else if p.verbose {
			log.Printf("Re-enqueued retry job %s to queue %s", job.JID, job.Queue)
		}
	}
}
