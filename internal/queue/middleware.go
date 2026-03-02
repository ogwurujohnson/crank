package queue

import (
	"context"
	"log"

	"github.com/ogwurujohnson/crank/internal/payload"
)

type MiddlewareFunc func(ctx context.Context, job *payload.Job, next func() error) error

type MiddlewareChain struct {
	middlewares []MiddlewareFunc
}

func NewMiddlewareChain() *MiddlewareChain {
	return &MiddlewareChain{
		middlewares: make([]MiddlewareFunc, 0),
	}
}

func (mc *MiddlewareChain) Add(middleware MiddlewareFunc) {
	mc.middlewares = append(mc.middlewares, middleware)
}

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

func AddMiddleware(middleware MiddlewareFunc) {
	globalMiddlewareChain.Add(middleware)
}

func GetMiddlewareChain() *MiddlewareChain {
	return globalMiddlewareChain
}

func LoggingMiddleware(ctx context.Context, job *payload.Job, next func() error) error {
	err := next()
	if err != nil {
		r := payload.GetDefaultRedactor()
		safeArgs := r.RedactArgs(job.Args)
		log.Printf("Job %s failed (args: %s): %v", job.JID, safeArgs, err)
	}
	return err
}
