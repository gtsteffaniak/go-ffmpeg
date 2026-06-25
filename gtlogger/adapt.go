// Package gtlogger adapts github.com/gtsteffaniak/go-logger for go-ffmpeg.
//
// go-logger's Logger interface already satisfies ffmpeg.Logger, so Adapt is
// optional documentation sugar. WithGroup is the recommended entry point when
// wiring a shared application logger into ffmpeg.New.
package gtlogger

import (
	ffmpeg "github.com/gtsteffaniak/go-ffmpeg"
	"github.com/gtsteffaniak/go-logger/logger"
)

const componentGroup = "ffmpeg"

// Adapt returns l for use as ffmpeg.Config.Logger.
func Adapt(l logger.Logger) ffmpeg.Logger {
	if l == nil {
		return ffmpeg.NopLogger()
	}
	return l
}

// WithGroup returns a grouped go-logger instance for ffmpeg diagnostics.
func WithGroup(l logger.Logger) ffmpeg.Logger {
	if l == nil {
		return ffmpeg.NopLogger()
	}
	return l.WithGroup(componentGroup)
}
