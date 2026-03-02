package payload

import (
	"fmt"
	"strings"
	"sync"
)

type Redactor interface {
	RedactArgs(args []interface{}) string
}

type NoopRedactor struct{}

func (NoopRedactor) RedactArgs(args []interface{}) string {
	return fmt.Sprintf("%v", args)
}

type MaskingRedactor struct{}

func (MaskingRedactor) RedactArgs(args []interface{}) string {
	if len(args) == 0 {
		return "[]"
	}
	return fmt.Sprintf("[REDACTED x%d]", len(args))
}

type FieldMaskingRedactor struct {
	Keys []string
}

func (f *FieldMaskingRedactor) RedactArgs(args []interface{}) string {
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

var DefaultRedactor Redactor = MaskingRedactor{}

var (
	defaultRedactor Redactor = DefaultRedactor
	redactorMu      sync.RWMutex
)

func SetDefaultRedactor(r Redactor) {
	redactorMu.Lock()
	defer redactorMu.Unlock()
	if r == nil {
		defaultRedactor = DefaultRedactor
		return
	}
	defaultRedactor = r
}

func GetDefaultRedactor() Redactor {
	redactorMu.RLock()
	defer redactorMu.RUnlock()
	if defaultRedactor == nil {
		return DefaultRedactor
	}
	return defaultRedactor
}
