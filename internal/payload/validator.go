package payload

import (
	"fmt"
	"regexp"
	"sync"
)

type Validator interface {
	Validate(job *Job) error
}

type ValidatorFunc func(job *Job) error

func (f ValidatorFunc) Validate(job *Job) error {
	return f(job)
}

func MaxArgsCount(n int) Validator {
	return ValidatorFunc(func(job *Job) error {
		if len(job.Args) > n {
			return fmt.Errorf("job args count %d exceeds max %d", len(job.Args), n)
		}
		return nil
	})
}

func ClassAllowlist(classes map[string]bool) Validator {
	return ValidatorFunc(func(job *Job) error {
		if !classes[job.Class] {
			return fmt.Errorf("job class '%s' not in allowlist", job.Class)
		}
		return nil
	})
}

func ClassPattern(pattern *regexp.Regexp) Validator {
	return ValidatorFunc(func(job *Job) error {
		if !pattern.MatchString(job.Class) {
			return fmt.Errorf("job class '%s' does not match allowed pattern", job.Class)
		}
		return nil
	})
}

func MaxPayloadSize(maxBytes int) Validator {
	return ValidatorFunc(func(job *Job) error {
		data, err := job.ToJSON()
		if err != nil {
			return err
		}
		if len(data) > maxBytes {
			return fmt.Errorf("job payload size %d exceeds max %d bytes", len(data), maxBytes)
		}
		return nil
	})
}

type ChainValidator []Validator

func (c ChainValidator) Validate(job *Job) error {
	for _, v := range c {
		if err := v.Validate(job); err != nil {
			return err
		}
	}
	return nil
}

var (
	defaultValidator Validator
	validatorMu      sync.RWMutex
)

func SetDefaultValidator(v Validator) {
	validatorMu.Lock()
	defer validatorMu.Unlock()
	defaultValidator = v
}

func GetDefaultValidator() Validator {
	validatorMu.RLock()
	defer validatorMu.RUnlock()
	return defaultValidator
}
