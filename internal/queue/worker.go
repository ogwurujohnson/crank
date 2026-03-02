package queue

import (
	"context"
	"fmt"
	"sync"
)

type Worker interface {
	Perform(ctx context.Context, args ...interface{}) error
}

type WorkerRegistry interface {
	GetWorker(className string) (Worker, error)
}

var (
	workers     = make(map[string]Worker)
	workersLock sync.RWMutex
)

func RegisterWorker(className string, worker Worker) {
	workersLock.Lock()
	defer workersLock.Unlock()
	workers[className] = worker
}

func GetWorker(className string) (Worker, error) {
	workersLock.RLock()
	defer workersLock.RUnlock()
	worker, exists := workers[className]
	if !exists {
		return nil, fmt.Errorf("worker class '%s' not found", className)
	}
	return worker, nil
}

func ListWorkers() []string {
	workersLock.RLock()
	defer workersLock.RUnlock()
	classes := make([]string, 0, len(workers))
	for className := range workers {
		classes = append(classes, className)
	}
	return classes
}
