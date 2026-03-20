package ui

import (
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"
	"sort"
	"strings"

	"github.com/ogwurujohnson/crank/internal/broker"
	"github.com/ogwurujohnson/crank/internal/payload"
	"github.com/ogwurujohnson/crank/internal/queue"
)

//go:embed static
var staticFS embed.FS

// DataSource supplies dashboard data from the running engine.
type DataSource interface {
	Stats() (*queue.Stats, error)
	PeekPending(queue string, limit int64) ([]*payload.Job, error)
	RetryScheduled(limit int64) ([]broker.RetrySchedule, error)
	DeadJobs(limit int64) ([]*payload.Job, error)
	QueueNames() []string
	WorkerClasses() []string
}

// Options configures the HTTP handler (mount path and copy).
type Options struct {
	// BasePath is the URL prefix where the UI is mounted (no trailing slash), e.g. "/crank".
	// Empty means root "/".
	BasePath string
	// Version label in the sidebar (e.g. "V2.4.0-STABLE").
	Version string
	// ProjectTitle appears under the sidebar brand (e.g. "PROJECT ALPHA").
	ProjectTitle string
}

// NewHandler serves the SPA and JSON API.
func NewHandler(src DataSource, opts Options) http.Handler {
	mux := http.NewServeMux()
	base := normalizeBase(opts.BasePath)

	mux.HandleFunc("GET /api/stats", jsonHandler(func(w http.ResponseWriter, r *http.Request) error {
		st, err := src.Stats()
		if err != nil {
			return err
		}
		return json.NewEncoder(w).Encode(st)
	}))
	mux.HandleFunc("GET /api/pending", jsonHandler(func(w http.ResponseWriter, r *http.Request) error {
		limit := parseLimit(r.FormValue("limit"), 100)
		q := strings.TrimSpace(r.FormValue("queue"))
		type row struct {
			Queue string         `json:"queue"`
			Job   *payload.Job   `json:"job"`
		}
		var rows []row
		if q != "" {
			jobs, err := src.PeekPending(q, limit)
			if err != nil {
				return err
			}
			for _, j := range jobs {
				rows = append(rows, row{Queue: q, Job: j})
			}
		} else {
			per := limit / int64(max(1, len(src.QueueNames())))
			if per < 1 {
				per = limit
			}
			for _, name := range src.QueueNames() {
				jobs, err := src.PeekPending(name, per)
				if err != nil {
					return err
				}
				for _, j := range jobs {
					rows = append(rows, row{Queue: name, Job: j})
				}
			}
			if int64(len(rows)) > limit {
				rows = rows[:limit]
			}
		}
		return json.NewEncoder(w).Encode(rows)
	}))
	mux.HandleFunc("GET /api/retries", jsonHandler(func(w http.ResponseWriter, r *http.Request) error {
		limit := parseLimit(r.FormValue("limit"), 100)
		list, err := src.RetryScheduled(limit)
		if err != nil {
			return err
		}
		return json.NewEncoder(w).Encode(list)
	}))
	mux.HandleFunc("GET /api/dead", jsonHandler(func(w http.ResponseWriter, r *http.Request) error {
		limit := parseLimit(r.FormValue("limit"), 100)
		jobs, err := src.DeadJobs(limit)
		if err != nil {
			return err
		}
		return json.NewEncoder(w).Encode(jobs)
	}))
	mux.HandleFunc("GET /api/registry", jsonHandler(func(w http.ResponseWriter, r *http.Request) error {
		names := src.WorkerClasses()
		sort.Strings(names)
		return json.NewEncoder(w).Encode(map[string][]string{"workers": names})
	}))
	mux.HandleFunc("GET /api/meta", jsonHandler(func(w http.ResponseWriter, r *http.Request) error {
		v := opts.Version
		if v == "" {
			v = "CRANK"
		}
		pt := opts.ProjectTitle
		if pt == "" {
			pt = "CONSUMER"
		}
		return json.NewEncoder(w).Encode(map[string]string{
			"version": v, "project": pt, "title": "THE ENGINEERING EDITORIAL",
		})
	}))

	sub, _ := fs.Sub(staticFS, "static")
	assetSrv := http.StripPrefix("/assets/", http.FileServer(http.FS(sub)))
	mux.Handle("GET /assets/", assetSrv)
	mux.Handle("HEAD /assets/", assetSrv)

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		data, err := staticFS.ReadFile("static/index.html")
		if err != nil {
			http.Error(w, "dashboard unavailable", http.StatusInternalServerError)
			return
		}
		html := strings.Replace(string(data), "__CRANK_BASE__", base, 1)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(html))
	})

	return mux
}

func normalizeBase(p string) string {
	p = strings.TrimSpace(p)
	if p == "" || p == "/" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return strings.TrimSuffix(p, "/") + "/"
}

func jsonHandler(fn func(http.ResponseWriter, *http.Request) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := fn(w, r); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

func parseLimit(s string, def int64) int64 {
	if s == "" {
		return def
	}
	var n int64
	for _, c := range s {
		if c < '0' || c > '9' {
			return def
		}
		n = n*10 + int64(c-'0')
		if n > 10000 {
			return 10000
		}
	}
	if n <= 0 {
		return def
	}
	return n
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
