package crank

import (
	"context"
	"net/http"
	"sort"
	"time"

	"github.com/ogwurujohnson/crank/internal/broker"
	"github.com/ogwurujohnson/crank/internal/payload"
	"github.com/ogwurujohnson/crank/internal/queue"
	"github.com/ogwurujohnson/crank/internal/ui"
)

// DashboardOptions configures the embedded engineering dashboard.
type DashboardOptions struct {
	// BasePath is where the UI is mounted on your server (no trailing slash), e.g. "/crank".
	// Empty means the dashboard is served at the root of the handler's URL space.
	BasePath string
	Version  string
	// ProjectTitle appears in the sidebar (e.g. "PROJECT ALPHA").
	ProjectTitle string
}

// DashboardHandler returns an http.Handler that serves the SPA and JSON API at the
// root of the mounted path (paths like /api/stats, /assets/..., and / for the shell).
//
// Mount on a subpath and strip the prefix so the inner paths match, and set BasePath
// to the same prefix for correct relative URLs in the browser:
//
//	mux.Handle("/crank/", http.StripPrefix("/crank", engine.DashboardHandler(crank.DashboardOptions{BasePath: "/crank"})))
//
// For a dedicated listener with no subpath, use BasePath "" (empty).
func (e *Engine) DashboardHandler(opts DashboardOptions) http.Handler {
	return ui.NewHandler(engineDataSource{e: e}, ui.Options{
		BasePath:     opts.BasePath,
		Version:      opts.Version,
		ProjectTitle: opts.ProjectTitle,
	})
}

// StartDashboard runs an HTTP server for the dashboard in a new goroutine.
// The server shuts down when ctx is cancelled. addr is passed to http.ListenAndServe (e.g. ":9090").
func (e *Engine) StartDashboard(ctx context.Context, addr string, opts DashboardOptions) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           e.DashboardHandler(opts),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shCtx)
	}()
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			e.cfg.Logger.Error("crank dashboard server error", "err", err)
		}
	}()
	return nil
}

type engineDataSource struct {
	e *Engine
}

func (d engineDataSource) Stats() (*queue.Stats, error) {
	return queue.GetStats(d.e.broker)
}

func (d engineDataSource) PeekPending(queueName string, limit int64) ([]*payload.Job, error) {
	return d.e.broker.PeekQueue(queueName, limit)
}

func (d engineDataSource) RetryScheduled(limit int64) ([]broker.RetrySchedule, error) {
	return d.e.broker.ListRetryScheduled(limit)
}

func (d engineDataSource) DeadJobs(limit int64) ([]*payload.Job, error) {
	return d.e.broker.GetDeadJobs(limit)
}

func (d engineDataSource) QueueNames() []string {
	seen := make(map[string]struct{})
	for _, qc := range d.e.cfg.Queues {
		if qc.Name != "" {
			seen[qc.Name] = struct{}{}
		}
	}
	names := make([]string, 0, len(seen))
	for n := range seen {
		names = append(names, n)
	}
	sort.Strings(names)
	if len(names) == 0 {
		return []string{"default"}
	}
	return names
}

func (d engineDataSource) WorkerClasses() []string {
	seen := make(map[string]struct{})
	d.e.registry.mu.RLock()
	for name := range d.e.registry.workers {
		seen[name] = struct{}{}
	}
	d.e.registry.mu.RUnlock()
	for _, name := range queue.ListWorkers() {
		seen[name] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
