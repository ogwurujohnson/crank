package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/quest/sidekiq-go"
)

// EmailWorker sends emails
type EmailWorker struct{}

func (w *EmailWorker) Perform(ctx context.Context, args ...interface{}) error {
	if len(args) < 1 {
		return fmt.Errorf("expected at least 1 argument")
	}

	userID, ok := args[0].(float64)
	if !ok {
		return fmt.Errorf("invalid user ID type")
	}

	fmt.Printf("Sending email to user %.0f\n", userID)
	time.Sleep(100 * time.Millisecond) // Simulate work
	fmt.Printf("Email sent to user %.0f\n", userID)
	return nil
}

// ReportWorker generates reports
type ReportWorker struct{}

func (w *ReportWorker) Perform(ctx context.Context, args ...interface{}) error {
	if len(args) < 1 {
		return fmt.Errorf("expected at least 1 argument")
	}

	reportID, ok := args[0].(float64)
	if !ok {
		return fmt.Errorf("invalid report ID type")
	}

	fmt.Printf("Generating report %.0f\n", reportID)
	time.Sleep(500 * time.Millisecond) // Simulate work
	fmt.Printf("Report %.0f generated\n", reportID)
	return nil
}

func main() {
	// Connect to Redis
	redis, err := sidekiq.NewRedisClient("redis://localhost:6379/0", 5*time.Second)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redis.Close()

	// Initialize client
	client := sidekiq.NewClient(redis)
	sidekiq.SetGlobalClient(client)

	// Register workers
	sidekiq.RegisterWorker("EmailWorker", &EmailWorker{})
	sidekiq.RegisterWorker("ReportWorker", &ReportWorker{})

	// Enqueue some jobs
	fmt.Println("Enqueueing jobs...")

	jid1, err := sidekiq.Enqueue("EmailWorker", "default", 123)
	if err != nil {
		log.Fatalf("Failed to enqueue job: %v", err)
	}
	fmt.Printf("Enqueued EmailWorker job: %s\n", jid1)

	jid2, err := sidekiq.Enqueue("ReportWorker", "low", 456)
	if err != nil {
		log.Fatalf("Failed to enqueue job: %v", err)
	}
	fmt.Printf("Enqueued ReportWorker job: %s\n", jid2)

	jid3, err := sidekiq.EnqueueWithOptions("EmailWorker", "critical", &sidekiq.JobOptions{
		Retry:     intPtr(3),
		Backtrace: boolPtr(true),
	}, 789)
	if err != nil {
		log.Fatalf("Failed to enqueue job: %v", err)
	}
	fmt.Printf("Enqueued EmailWorker job with options: %s\n", jid3)

	fmt.Println("\nJobs enqueued. Start the worker process to process them:")
	fmt.Println("  go run cmd/sidekiq/main.go -C config/sidekiq.yml")
}

func intPtr(i int) *int {
	return &i
}

func boolPtr(b bool) *bool {
	return &b
}

