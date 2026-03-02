package queue

import "github.com/ogwurujohnson/crank/internal/config"

type Logger = config.Logger

type nopLogger struct{}

func (nopLogger) Debug(msg string, args ...any) {}
func (nopLogger) Info(msg string, args ...any)  {}
func (nopLogger) Warn(msg string, args ...any)  {}
func (nopLogger) Error(msg string, args ...any) {}

var nop = nopLogger{}

func NopLogger() Logger { return nop }
