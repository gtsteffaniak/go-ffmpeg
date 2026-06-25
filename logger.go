package ffmpeg

import (
	"log/slog"
)

// Logger is the logging interface used by the library.
//
// Pass an implementation through Config.Logger when creating a Service.
// Any logger that implements these four methods works, including
// github.com/gtsteffaniak/go-logger/logger.Logger and *slog.Logger
// (via FromSlog).
//
// For library code, prefer explicit injection over global loggers.
type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

type slogLogger struct {
	l *slog.Logger
}

func (s slogLogger) Debug(msg string, args ...any) { s.l.Debug(msg, args...) }
func (s slogLogger) Info(msg string, args ...any)  { s.l.Info(msg, args...) }
func (s slogLogger) Warn(msg string, args ...any)  { s.l.Warn(msg, args...) }
func (s slogLogger) Error(msg string, args ...any) { s.l.Error(msg, args...) }

func defaultLogger() Logger {
	return FromSlog(slog.Default())
}

// FromSlog wraps a slog.Logger for use as Config.Logger.
func FromSlog(l *slog.Logger) Logger {
	if l == nil {
		return NopLogger()
	}
	return slogLogger{l: l}
}

type nopLogger struct{}

func (nopLogger) Debug(string, ...any) {}
func (nopLogger) Info(string, ...any)  {}
func (nopLogger) Warn(string, ...any)  {}
func (nopLogger) Error(string, ...any) {}

// NopLogger returns a logger that discards all output.
func NopLogger() Logger { return nopLogger{} }

type groupLogger struct {
	base  Logger
	group string
}

func (g groupLogger) Debug(msg string, args ...any) {
	g.base.Debug(msg, append([]any{"group", g.group}, args...)...)
}
func (g groupLogger) Info(msg string, args ...any) {
	g.base.Info(msg, append([]any{"group", g.group}, args...)...)
}
func (g groupLogger) Warn(msg string, args ...any) {
	g.base.Warn(msg, append([]any{"group", g.group}, args...)...)
}
func (g groupLogger) Error(msg string, args ...any) {
	g.base.Error(msg, append([]any{"group", g.group}, args...)...)
}

// WithGroup returns a logger that tags all records with group.
// When the underlying logger supports its own WithGroup method (for example
// github.com/gtsteffaniak/go-logger), that implementation is used instead.
func WithGroup(l Logger, group string) Logger {
	if l == nil {
		return NopLogger()
	}
	type grouper interface {
		WithGroup(string) interface {
			Debug(msg string, args ...any)
			Info(msg string, args ...any)
			Warn(msg string, args ...any)
			Error(msg string, args ...any)
		}
	}
	if g, ok := l.(grouper); ok {
		return g.WithGroup(group)
	}
	return groupLogger{base: l, group: group}
}
