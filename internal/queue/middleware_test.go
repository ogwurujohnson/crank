package queue

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/ogwurujohnson/crank/internal/payload"
)

func TestMiddlewareChain_Execute_Order(t *testing.T) {
	c := qt.New(t)

	var got []string
	mc := NewMiddlewareChain()
	mc.Add(func(ctx context.Context, job *payload.Job, next func() error) error {
		got = append(got, "a1")
		err := next()
		got = append(got, "a2")
		return err
	})
	mc.Add(func(ctx context.Context, job *payload.Job, next func() error) error {
		got = append(got, "b1")
		err := next()
		got = append(got, "b2")
		return err
	})

	err := mc.Execute(context.Background(), payload.NewJob("W", "q"), func() error {
		got = append(got, "final")
		return nil
	})
	c.Assert(err, qt.IsNil)
	c.Assert(got, qt.DeepEquals, []string{"a1", "b1", "final", "b2", "a2"})
}
