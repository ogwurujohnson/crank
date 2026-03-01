package queue

import (
	"context"
	"log"

	"github.com/ogwurujohnson/crank/internal/payload"
)

// MiddlewareFunc defines a middleware function
type MiddlewareFunc func(ctx context.Context, job *payload.Job, next func() error) error

// MiddlewareChain manages middleware execution
type MiddlewareChain struct {
	middlewares []MiddlewareFunc
}

// NewMiddlewareChain creates a new middleware chain
func NewMiddlewareChain() *MiddlewareChain {
	return &MiddlewareChain{
		middlewares: make([]MiddlewareFunc, 0),
	}
}

// Add adds a middleware to the chain
func (mc *MiddlewareChain) Add(middleware MiddlewareFunc) {
	mc.middlewares = append(mc.middlewares, middleware)
}

// Execute executes the middleware chain
func (mc *MiddlewareChain) Execute(ctx context.Context, job *payload.Job, final func() error) error {
	if len(mc.middlewares) == 0 {
		return final()
	}

	var index int
	var next func() error
	next = func() error {
		if index >= len(mc.middlewares) {
			return final()
		}
		mw := mc.middlewares[index]
		index++
		return mw(ctx, job, next)
	}

	return next()
}

var globalMiddlewareChain = NewMiddlewareChain()

// AddMiddleware adds middleware to the global chain
func AddMiddleware(middleware MiddlewareFunc) {
	globalMiddlewareChain.Add(middleware)
}

// GetMiddlewareChain returns the global middleware chain
func GetMiddlewareChain() *MiddlewareChain {
	return globalMiddlewareChain
}

// LoggingMiddleware logs job execution with redacted args on failure
func LoggingMiddleware(ctx context.Context, job *payload.Job, next func() error) error {
	err := next()
	if err != nil {
		r := payload.GetDefaultRedactor()
		safeArgs := r.RedactArgs(job.Args)
		log.Printf("Job %s failed (args: %s): %v", job.JID, safeArgs, err)
	}
	return err
}
