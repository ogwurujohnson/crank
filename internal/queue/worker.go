package queue

import (
	"context"
	"fmt"
	"sync"
)

// Worker interface defines the contract for job workers
type Worker interface {
	Perform(ctx context.Context, args ...interface{}) error
}

var (
	workers     = make(map[string]Worker)
	workersLock  sync.RWMutex
)

// RegisterWorker registers a worker class
// className is the name of the worker class, e.g. "EmailWorker"
// should be renamed to interfaceName for clarity.
func RegisterWorker(className string, worker Worker) {
	workersLock.Lock()
	defer workersLock.Unlock()
	workers[className] = worker
}

// GetWorker retrieves a registered worker
func GetWorker(className string) (Worker, error) {
	workersLock.RLock()
	defer workersLock.RUnlock()
	worker, exists := workers[className]
	if !exists {
		return nil, fmt.Errorf("worker class '%s' not found", className)
	}
	return worker, nil
}

// ListWorkers returns all registered worker class names
func ListWorkers() []string {
	workersLock.RLock()
	defer workersLock.RUnlock()
	classes := make([]string, 0, len(workers))
	for className := range workers {
		classes = append(classes, className)
	}
	return classes
}
