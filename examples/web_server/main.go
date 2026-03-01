package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/quest/crank"
	"github.com/quest/crank/web"
)

// WebEmailWorker sends emails (for web server example)
type WebEmailWorker struct{}

func (w *WebEmailWorker) Perform(ctx context.Context, args ...interface{}) error {
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

func main() {
	// Connect to Redis
	redis, err := crank.NewRedisClient("redis://localhost:6379/0", 5*time.Second)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redis.Close()

	// Initialize client
	client := crank.NewClient(redis)
	crank.SetGlobalClient(client)

	// Register workers
	crank.RegisterWorker("WebEmailWorker", &WebEmailWorker{})

	// Setup HTTP server with Crank Web UI
	router := mux.NewRouter()
	web.Mount(router, "/crank", redis)

	// API endpoint to enqueue jobs
	router.HandleFunc("/api/jobs", func(w http.ResponseWriter, r *http.Request) {
		userID := r.URL.Query().Get("user_id")
		if userID == "" {
			http.Error(w, "user_id parameter required", http.StatusBadRequest)
			return
		}

		var userIDFloat float64
		if _, err := fmt.Sscanf(userID, "%f", &userIDFloat); err != nil {
			http.Error(w, "invalid user_id", http.StatusBadRequest)
			return
		}

		jid, err := crank.Enqueue("WebEmailWorker", "default", userIDFloat)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"jid":"%s","status":"enqueued"}`, jid)
	}).Methods("POST")

	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `
			<h1>Crank Example</h1>
			<p><a href="/crank">View Crank Web UI</a></p>
			<p>Enqueue a job: <code>curl -X POST "http://localhost:8080/api/jobs?user_id=123"</code></p>
		`)
	})

	log.Println("Server starting on :8080")
	log.Println("Crank Web UI: http://localhost:8080/crank")
	log.Fatal(http.ListenAndServe(":8080", router))
}
