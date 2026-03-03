package payload

import (
	"regexp"
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestMaxArgsCount(t *testing.T) {
	c := qt.New(t)
	v := MaxArgsCount(2)
	job := NewJob("W", "q", 1, 2)
	c.Assert(v.Validate(job), qt.IsNil)
	job.Args = append(job.Args, 3)
	c.Assert(v.Validate(job), qt.IsNotNil)
}

func TestClassAllowlist(t *testing.T) {
	c := qt.New(t)
	v := ClassAllowlist(map[string]bool{"A": true, "B": true})
	job := NewJob("A", "q")
	c.Assert(v.Validate(job), qt.IsNil)
	job.Class = "C"
	c.Assert(v.Validate(job), qt.IsNotNil)
}

func TestClassPattern(t *testing.T) {
	c := qt.New(t)
	re := regexp.MustCompile(`^[A-Za-z0-9_]+$`)
	v := ClassPattern(re)
	job := NewJob("ValidWorker", "q")
	c.Assert(v.Validate(job), qt.IsNil)
	job.Class = "bad-class"
	c.Assert(v.Validate(job), qt.IsNotNil)
}

func TestMaxPayloadSize(t *testing.T) {
	c := qt.New(t)
	v := MaxPayloadSize(500)
	job := NewJob("W", "q", "small")
	c.Assert(v.Validate(job), qt.IsNil)
	job.Args = []interface{}{string(make([]byte, 600))}
	c.Assert(v.Validate(job), qt.IsNotNil)
}

func TestChainValidator(t *testing.T) {
	c := qt.New(t)
	chain := ChainValidator{
		MaxArgsCount(2),
		ClassAllowlist(map[string]bool{"W": true}),
	}
	job := NewJob("W", "q", 1, 2)
	c.Assert(chain.Validate(job), qt.IsNil)
	job.Class = "X"
	c.Assert(chain.Validate(job), qt.IsNotNil)
	job.Class = "W"
	job.Args = []interface{}{1, 2, 3}
	c.Assert(chain.Validate(job), qt.IsNotNil)
}
