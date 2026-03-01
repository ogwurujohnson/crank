package sidekiq

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// Processor manages job processing with a worker pool
type Processor struct {
	config      *Config
	redis       *RedisClient
	queues      []string
	workers     chan struct{}
	wg          sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc
	retryTicker *time.Ticker
	verbose     bool
}

// NewProcessor creates a new processor instance
func NewProcessor(config *Config, redis *RedisClient) (*Processor, error) {
	// Build queue list from config (with weights)
	queues := make([]string, 0)
	for _, qc := range config.Queues {
		// Add queue name multiple times based on weight
		for i := 0; i < qc.Weight; i++ {
			queues = append(queues, qc.Name)
		}
	}

	// If no queues configured, use default
	if len(queues) == 0 {
		queues = []string{"default"}
	}

	ctx, cancel := context.WithCancel(context.Background())

	p := &Processor{
		config:      config,
		redis:       redis,
		queues:      queues,
		workers:     make(chan struct{}, config.Concurrency),
		ctx:         ctx,
		cancel:      cancel,
		retryTicker: time.NewTicker(5 * time.Second),
		verbose:     config.Verbose,
	}

	return p, nil
}

// Start begins processing jobs
func (p *Processor) Start() error {
	if p.verbose {
		log.Printf("Starting Sidekiq processor with concurrency=%d, queues=%v", p.config.Concurrency, p.queues)
	}

	// Start worker goroutines
	for i := 0; i < p.config.Concurrency; i++ {
		p.wg.Add(1)
		go p.worker()
	}

	// Start retry processor
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

// worker is the main worker loop
func (p *Processor) worker() {
	defer p.wg.Done()

	for {
		select {
		case <-p.ctx.Done():
			return
		default:
			// Acquire worker slot
			p.workers <- struct{}{}

			// Dequeue job with timeout
			job, queue, err := p.redis.Dequeue(p.queues, 1*time.Second)
			if err != nil {
				<-p.workers
				if p.verbose {
					log.Printf("Error dequeuing job: %v", err)
				}
				continue
			}

			if job == nil {
				<-p.workers
				continue // Timeout, try again
			}

			// Process job in goroutine to free up worker slot
			go func(j *Job, q string) {
				defer func() { <-p.workers }()
				p.processJob(j, q)
			}(job, queue)
		}
	}
}

// processJob processes a single job
func (p *Processor) processJob(job *Job, queue string) {
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), p.config.GetTimeout())
	defer cancel()

	if p.verbose {
		log.Printf("Processing %s", job)
	}

	// Get worker
	worker, err := GetWorker(job.Class)
	if err != nil {
		log.Printf("Error getting worker '%s': %v", job.Class, err)
		p.handleFailure(job, fmt.Errorf("worker not found: %w", err))
		return
	}

	// Execute worker through middleware chain
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

// handleFailure handles job failures with retry logic
func (p *Processor) handleFailure(job *Job, err error) {
	job.RetryCount++

	if job.RetryCount <= job.Retry {
		// Calculate exponential backoff
		backoff := time.Duration(1<<uint(job.RetryCount)) * time.Second
		retryAt := time.Now().Add(backoff)

		if p.verbose {
			log.Printf("Scheduling retry for job %s (attempt %d/%d) at %v", job.JID, job.RetryCount, job.Retry, retryAt)
		}

		if err := p.redis.AddToRetry(job, retryAt); err != nil {
			log.Printf("Error adding job to retry: %v", err)
		}
	} else {
		// Max retries exceeded, move to dead
		if p.verbose {
			log.Printf("Job %s exceeded max retries, moving to dead queue", job.JID)
		}

		if err := p.redis.AddToDead(job); err != nil {
			log.Printf("Error adding job to dead queue: %v", err)
		}
	}
}

// retryProcessor processes retry jobs
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

// processRetries processes jobs ready for retry
func (p *Processor) processRetries() {
	jobs, err := p.redis.GetRetryJobs(100)
	if err != nil {
		if p.verbose {
			log.Printf("Error getting retry jobs: %v", err)
		}
		return
	}

	for _, job := range jobs {
		// Remove from retry set
		if err := p.redis.RemoveFromRetry(job); err != nil {
			if p.verbose {
				log.Printf("Error removing job from retry: %v", err)
			}
			continue
		}

		// Re-enqueue to original queue
		if err := p.redis.Enqueue(job.Queue, job); err != nil {
			if p.verbose {
				log.Printf("Error re-enqueueing retry job: %v", err)
			}
			// Put it back in retry
			p.redis.AddToRetry(job, time.Now().Add(1*time.Minute))
		} else if p.verbose {
			log.Printf("Re-enqueued retry job %s to queue %s", job.JID, job.Queue)
		}
	}
}

