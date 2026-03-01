package payload

import (
	"regexp"
	"testing"
)

func TestMaxArgsCount(t *testing.T) {
	v := MaxArgsCount(2)
	job := NewJob("W", "q", 1, 2)
	if err := v.Validate(job); err != nil {
		t.Errorf("expected nil: %v", err)
	}
	job.Args = append(job.Args, 3)
	if err := v.Validate(job); err == nil {
		t.Error("expected error for 3 args")
	}
}

func TestClassAllowlist(t *testing.T) {
	v := ClassAllowlist(map[string]bool{"A": true, "B": true})
	job := NewJob("A", "q")
	if err := v.Validate(job); err != nil {
		t.Errorf("expected nil: %v", err)
	}
	job.Class = "C"
	if err := v.Validate(job); err == nil {
		t.Error("expected error for class C")
	}
}

func TestClassPattern(t *testing.T) {
	re := regexp.MustCompile(`^[A-Za-z0-9_]+$`)
	v := ClassPattern(re)
	job := NewJob("ValidWorker", "q")
	if err := v.Validate(job); err != nil {
		t.Errorf("expected nil: %v", err)
	}
	job.Class = "bad-class"
	if err := v.Validate(job); err == nil {
		t.Error("expected error for bad-class")
	}
}

func TestMaxPayloadSize(t *testing.T) {
	v := MaxPayloadSize(500)
	job := NewJob("W", "q", "small")
	if err := v.Validate(job); err != nil {
		t.Errorf("expected nil: %v", err)
	}
	job.Args = []interface{}{string(make([]byte, 600))}
	if err := v.Validate(job); err == nil {
		t.Error("expected error for large payload")
	}
}

func TestChainValidator(t *testing.T) {
	chain := ChainValidator{
		MaxArgsCount(2),
		ClassAllowlist(map[string]bool{"W": true}),
	}
	job := NewJob("W", "q", 1, 2)
	if err := chain.Validate(job); err != nil {
		t.Errorf("expected nil: %v", err)
	}
	job.Class = "X"
	if err := chain.Validate(job); err == nil {
		t.Error("expected error for class X")
	}
	job.Class = "W"
	job.Args = []interface{}{1, 2, 3}
	if err := chain.Validate(job); err == nil {
		t.Error("expected error for 3 args")
	}
}
