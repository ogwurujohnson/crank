package ui

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ogwurujohnson/crank/internal/broker"
	"github.com/ogwurujohnson/crank/internal/payload"
	"github.com/ogwurujohnson/crank/internal/queue"
)

type stubSrc struct{}

func (stubSrc) Stats() (*queue.Stats, error) {
	return &queue.Stats{Processed: 1, Retry: 2, Dead: 0, Queues: map[string]int64{"default": 3}}, nil
}

func (stubSrc) PeekPending(string, int64) ([]*payload.Job, error) {
	return []*payload.Job{payload.NewJob("W", "default", 1)}, nil
}

func (stubSrc) RetryScheduled(int64) ([]broker.RetrySchedule, error) { return nil, nil }

func (stubSrc) DeadJobs(int64) ([]*payload.Job, error) { return nil, nil }

func (stubSrc) QueueNames() []string { return []string{"default"} }

func (stubSrc) WorkerClasses() []string { return []string{"W"} }

func TestNewHandler_GET_index_and_api(t *testing.T) {
	h := NewHandler(stubSrc{}, Options{})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	for _, path := range []string{"/", "/api/stats", "/api/pending", "/api/registry", "/api/meta"} {
		res, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusOK {
			t.Errorf("GET %s: status %d", path, res.StatusCode)
		}
	}
}

func TestNewHandler_assets(t *testing.T) {
	h := NewHandler(stubSrc{}, Options{})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	res, err := http.Get(srv.URL + "/assets/app.css")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET /assets/app.css: %d", res.StatusCode)
	}
}

func TestListRetryScheduled_memory(t *testing.T) {
	b := broker.NewInMemoryBroker()
	t.Cleanup(func() { _ = b.Close() })
	j := payload.NewJob("x", "default", 1)
	at := time.Now().Add(time.Minute)
	if err := b.AddToRetry(j, at); err != nil {
		t.Fatal(err)
	}
	list, err := b.ListRetryScheduled(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Job.JID != j.JID {
		t.Fatalf("got %+v", list)
	}
}
