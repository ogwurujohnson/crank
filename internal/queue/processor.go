package queue

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/ogwurujohnson/crank/internal/broker"
	"github.com/ogwurujohnson/crank/internal/config"
	"github.com/ogwurujohnson/crank/internal/payload"
)

// jobMsg carries a dequeued job and its queue name to a worker.
type jobMsg struct {
	job   *payload.Job
	queue string
}

// Processor manages job processing with a worker pool. Backpressure: a single
// fetcher dequeues and sends to an unbuffered channel; N workers receive and
// process. When all workers are busy, the fetcher blocks on send, so no new
// jobs are pulled until a worker is free (the crank slows down).
type Processor struct {
	cfg         *config.Config
	broker      broker.Broker
	registry    WorkerRegistry // if nil, uses global GetWorker
	queues      []string
	jobCh       chan jobMsg // unbuffered: send blocks when all workers busy
	wg          sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc
	retryTicker *time.Ticker
	verbose     bool
}

// NewProcessor creates a new processor instance. If registry is nil, the global
// worker registry (RegisterWorker/GetWorker) is used.
func NewProcessor(cfg *config.Config, b broker.Broker, registry WorkerRegistry) (*Processor, error) {
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
		registry:    registry,
		queues:      queues,
		jobCh:       make(chan jobMsg), // unbuffered for backpressure
		ctx:         ctx,
		cancel:      cancel,
		retryTicker: time.NewTicker(5 * time.Second),
		verbose:     cfg.Verbose,
	}

	return p, nil
}

// Start begins processing jobs. One fetcher goroutine dequeues and sends to
// jobCh; concurrency worker goroutines receive and process. Backpressure:
// fetcher blocks on send when all workers are busy.
func (p *Processor) Start() error {
	if p.verbose {
		log.Printf("Starting Crank processor with concurrency=%d, queues=%v", p.cfg.Concurrency, p.queues)
	}

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

// Stop gracefully stops the processor
func (p *Processor) Stop() {
	if p.verbose {
		log.Println("Stopping Crank processor...")
	}

	p.cancel()
	p.retryTicker.Stop()
	p.wg.Wait()

	if p.verbose {
		log.Println("Crank processor stopped")
	}
}

// fetcher dequeues jobs and sends them to jobCh. When all workers are busy,
// the send blocks (backpressure: crank slows down). Closes jobCh on exit so
// workers can shut down.
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
				if p.verbose {
					log.Printf("Error dequeuing job: %v", err)
				}
				continue
			}
			if job == nil {
				continue
			}

			// Send blocks when all workers busy (backpressure). On shutdown, put job back.
			select {
			case <-p.ctx.Done():
				_ = p.broker.Enqueue(queue, job)
				return
			case p.jobCh <- jobMsg{job: job, queue: queue}:
			}
		}
	}
}

// worker receives from jobCh and processes jobs. Blocks on receive when idle,
// so the fetcher only dequeues when a worker is free.
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

	var worker Worker
	var err error
	if p.registry != nil {
		worker, err = p.registry.GetWorker(job.Class)
	} else {
		worker, err = GetWorker(job.Class)
	}
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
