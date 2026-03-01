package payload

import (
	"fmt"
	"strings"
	"sync"
)

// Redactor produces a log-safe representation of job arguments.
// Use to prevent PII, passwords, or other sensitive data from appearing in logs.
type Redactor interface {
	RedactArgs(args []interface{}) string
}

// NoopRedactor returns args as-is (use only when args are known safe)
type NoopRedactor struct{}

func (NoopRedactor) RedactArgs(args []interface{}) string {
	return fmt.Sprintf("%v", args)
}

// MaskingRedactor replaces all arguments with [REDACTED]
type MaskingRedactor struct{}

func (MaskingRedactor) RedactArgs(args []interface{}) string {
	if len(args) == 0 {
		return "[]"
	}
	return fmt.Sprintf("[REDACTED x%d]", len(args))
}

// FieldMaskingRedactor redacts map/struct values for given keys (e.g. "password", "token")
type FieldMaskingRedactor struct {
	Keys []string
}

func (f *FieldMaskingRedactor) RedactArgs(args []interface{}) string {
	// Simple implementation: mask if any arg is a map containing sensitive keys
	parts := make([]string, len(args))
	for i, a := range args {
		if m, ok := a.(map[string]interface{}); ok {
			masked := make(map[string]interface{})
			for k, v := range m {
				masked[k] = v
				for _, sk := range f.Keys {
					if strings.EqualFold(k, sk) {
						masked[k] = "[REDACTED]"
						break
					}
				}
			}
			parts[i] = fmt.Sprintf("%v", masked)
		} else {
			parts[i] = fmt.Sprintf("%v", a)
		}
	}
	return fmt.Sprintf("%v", parts)
}

// DefaultRedactor is the default redactor (masking) for production safety
var DefaultRedactor Redactor = MaskingRedactor{}

var (
	defaultRedactor Redactor = DefaultRedactor
	redactorMu      sync.RWMutex
)

// SetDefaultRedactor sets the global redactor for job argument logging
func SetDefaultRedactor(r Redactor) {
	redactorMu.Lock()
	defer redactorMu.Unlock()
	if r == nil {
		defaultRedactor = DefaultRedactor
		return
	}
	defaultRedactor = r
}

// GetDefaultRedactor returns the current default redactor
func GetDefaultRedactor() Redactor {
	redactorMu.RLock()
	defer redactorMu.RUnlock()
	if defaultRedactor == nil {
		return DefaultRedactor
	}
	return defaultRedactor
}
