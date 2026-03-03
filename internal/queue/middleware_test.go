package queue

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/ogwurujohnson/crank/internal/payload"
)

func TestChain_Wrap_Order(t *testing.T) {
	c := qt.New(t)

	var got []string
	chain := NewChain()

	chain.Use(func(next Handler) Handler {
		return func(ctx context.Context, job *payload.Job) error {
			got = append(got, "a1")
			err := next(ctx, job)
			got = append(got, "a2")
			return err
		}
	})
	chain.Use(func(next Handler) Handler {
		return func(ctx context.Context, job *payload.Job) error {
			got = append(got, "b1")
			err := next(ctx, job)
			got = append(got, "b2")
			return err
		}
	})

	h := chain.Wrap(func(ctx context.Context, job *payload.Job) error {
		got = append(got, "final")
		return nil
	})

	err := h(context.Background(), payload.NewJob("W", "q"))
	c.Assert(err, qt.IsNil)
	c.Assert(got, qt.DeepEquals, []string{"a1", "b1", "final", "b2", "a2"})
}
