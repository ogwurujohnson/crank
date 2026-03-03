package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ogwurujohnson/crank"
)

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
	redis, err := crank.NewRedisClient("redis://localhost:6379/0", 5*time.Second)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redis.Close()

	client := crank.NewClient(redis)
	crank.SetGlobalClient(client)
	crank.RegisterWorker("EmailWorker", &EmailWorker{})
	crank.RegisterWorker("ReportWorker", &ReportWorker{})

	fmt.Println("Enqueueing jobs...")

	jid1, err := crank.Enqueue("EmailWorker", "default", 123)
	if err != nil {
		log.Fatalf("Failed to enqueue job: %v", err)
	}
	fmt.Printf("Enqueued EmailWorker job: %s\n", jid1)

	jid2, err := crank.Enqueue("ReportWorker", "low", 456)
	if err != nil {
		log.Fatalf("Failed to enqueue job: %v", err)
	}
	fmt.Printf("Enqueued ReportWorker job: %s\n", jid2)

	jid3, err := crank.EnqueueWithOptions("EmailWorker", "critical", &crank.JobOptions{
		Retry:     intPtr(3),
		Backtrace: boolPtr(true),
	}, 789)
	if err != nil {
		log.Fatalf("Failed to enqueue job: %v", err)
	}
	fmt.Printf("Enqueued EmailWorker job with options: %s\n", jid3)

	fmt.Println("\nJobs enqueued. Start the worker process to process them:")
	fmt.Println("  go run ./cmd/crank/ -C config/crank.yml")
}

func intPtr(i int) *int {
	return &i
}

func boolPtr(b bool) *bool {
	return &b
}
