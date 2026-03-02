package queue

import "github.com/ogwurujohnson/crank/internal/config"

// Logger is compatible with slog-style logging (msg string, args ...any).
// Alias of config.Logger so queue code can use queue.Logger; use NopLogger() as safe default.
type Logger = config.Logger

type nopLogger struct{}

func (nopLogger) Debug(msg string, args ...any) {}
func (nopLogger) Info(msg string, args ...any)  {}
func (nopLogger) Warn(msg string, args ...any)  {}
func (nopLogger) Error(msg string, args ...any) {}

var nop = nopLogger{}

// NopLogger returns a Logger that does nothing. Safe default when cfg.Logger is nil.
func NopLogger() Logger { return nop }
