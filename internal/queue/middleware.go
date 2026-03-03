package queue

import (
	"context"
	"fmt"
	"runtime"

	"github.com/ogwurujohnson/crank/internal/payload"
)

type Handler func(ctx context.Context, job *payload.Job) error

type Middleware func(next Handler) Handler

type Chain struct {
	middlewares []Middleware
}

func NewChain(ms ...Middleware) *Chain {
	return &Chain{
		middlewares: ms,
	}
}

func (c *Chain) Use(m ...Middleware) {
	c.middlewares = append(c.middlewares, m...)
}

func (c *Chain) Wrap(final Handler) Handler {
	h := final
	for i := len(c.middlewares) - 1; i >= 0; i-- {
		h = c.middlewares[i](h)
	}
	return h
}

func LoggingMiddleware(logger Logger) Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context, job *payload.Job) error {
			err := next(ctx, job)
			if err != nil {
				r := payload.GetDefaultRedactor()
				safeArgs := r.RedactArgs(job.Args)
				logger.Error("job failed", "jid", job.JID, "args", safeArgs, "err", err)
			}
			return err
		}
	}
}

func RecoveryMiddleware(logger Logger) Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context, job *payload.Job) (err error) {
			defer func() {
				if r := recover(); r != nil {
					buf := make([]byte, 64<<10)
					n := runtime.Stack(buf, false)
					stack := string(buf[:n])

					logger.Error("panic recovered in job handler",
						"jid", job.JID,
						"panic", r,
						"stack", stack,
					)

					err = fmt.Errorf("panic: %v", r)
				}
			}()

			return next(ctx, job)
		}
	}
}
