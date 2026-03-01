package web

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/ogwurujohnson/crank"
)

var (
	broker crank.Broker
)

// Mount mounts the Crank web UI on the given router
func Mount(router *mux.Router, path string, b crank.Broker) {
	broker = b
	subrouter := router.PathPrefix(path).Subrouter()

	subrouter.HandleFunc("", indexHandler).Methods("GET")
	subrouter.HandleFunc("/", indexHandler).Methods("GET")
	subrouter.HandleFunc("/stats", statsHandler).Methods("GET")
	subrouter.HandleFunc("/queues", queuesHandler).Methods("GET")
	subrouter.HandleFunc("/queues/{queue}/clear", clearQueueHandler).Methods("POST")
	subrouter.HandleFunc("/retries", retriesHandler).Methods("GET")
	subrouter.HandleFunc("/dead", deadHandler).Methods("GET")

	// Serve static assets
	subrouter.PathPrefix("/static/").Handler(http.StripPrefix(path+"/static/", http.FileServer(http.Dir("web/static/"))))
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	tmpl := `<!DOCTYPE html>
<html>
<head>
	<title>Crank</title>
	<style>
		body { font-family: Arial, sans-serif; margin: 20px; background: #f5f5f5; }
		.container { max-width: 1200px; margin: 0 auto; background: white; padding: 20px; border-radius: 5px; }
		.stats { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 20px; margin: 20px 0; }
		.stat-card { background: #f8f9fa; padding: 15px; border-radius: 5px; border-left: 4px solid #007bff; }
		.stat-value { font-size: 2em; font-weight: bold; color: #007bff; }
		.stat-label { color: #666; margin-top: 5px; }
		.queue-list { margin: 20px 0; }
		.queue-item { display: flex; justify-content: space-between; padding: 10px; background: #f8f9fa; margin: 5px 0; border-radius: 3px; }
		.btn { padding: 8px 16px; background: #dc3545; color: white; border: none; border-radius: 3px; cursor: pointer; }
		.btn:hover { background: #c82333; }
		h1 { color: #333; }
		h2 { color: #666; margin-top: 30px; }
	</style>
</head>
<body>
	<div class="container">
		<h1>Crank</h1>
		<div id="stats"></div>
		<h2>Queues</h2>
		<div id="queues"></div>
	</div>
	<script>
		var base = window.location.pathname.replace(/\/$/, '');
		function loadStats() {
			fetch(base + '/stats')
				.then(r => r.json())
				.then(data => {
					const html = '<div class="stats">' +
						'<div class="stat-card"><div class="stat-value">' + data.processed + '</div><div class="stat-label">Processed</div></div>' +
						'<div class="stat-card"><div class="stat-value">' + data.retry + '</div><div class="stat-label">Retry</div></div>' +
						'<div class="stat-card"><div class="stat-value">' + data.dead + '</div><div class="stat-label">Dead</div></div>' +
						'</div>';
					document.getElementById('stats').innerHTML = html;
				});
		}
		function loadQueues() {
			fetch(base + '/queues')
				.then(r => r.json())
				.then(data => {
					let html = '<div class="queue-list">';
					for (const [name, size] of Object.entries(data)) {
						html += '<div class="queue-item">' +
							'<span><strong>' + name + '</strong>: ' + size + ' jobs</span>' +
							'<button class="btn" onclick="clearQueue(\'' + name + '\')">Clear</button>' +
							'</div>';
					}
					html += '</div>';
					document.getElementById('queues').innerHTML = html;
				});
		}
		function clearQueue(name) {
			if (confirm('Clear queue ' + name + '?')) {
				fetch(base + '/queues/' + name + '/clear', {method: 'POST'})
					.then(() => loadQueues());
			}
		}
		loadStats();
		loadQueues();
		setInterval(loadStats, 5000);
		setInterval(loadQueues, 5000);
	</script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(tmpl))
}

func statsHandler(w http.ResponseWriter, r *http.Request) {
	stats, err := crank.GetStats(broker)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func queuesHandler(w http.ResponseWriter, r *http.Request) {
	stats, err := crank.GetStats(broker)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats.Queues)
}

func clearQueueHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	queueName := vars["queue"]

	queue := crank.NewQueue(queueName, broker)
	if err := queue.Clear(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Queue %s cleared", queueName)
}

func retriesHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement retry job listing
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"count": 0, "jobs": []interface{}{}})
}

func deadHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement dead job listing
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"count": 0, "jobs": []interface{}{}})
}
