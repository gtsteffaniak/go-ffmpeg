package gtlogger

import (
	"testing"

	ffmpeg "github.com/gtsteffaniak/go-ffmpeg"
	"github.com/gtsteffaniak/go-logger/logger"
)

func TestAdaptNilReturnsNop(t *testing.T) {
	if Adapt(nil) == nil {
		t.Fatal("expected nop logger")
	}
}

func TestWithGroupNilReturnsNop(t *testing.T) {
	if WithGroup(nil) == nil {
		t.Fatal("expected nop logger")
	}
}

func TestAdaptSatisfiesFFmpegLogger(t *testing.T) {
	log, err := logger.NewLogger(logger.JsonConfig{Levels: "INFO"})
	if err != nil {
		t.Fatal(err)
	}
	var _ ffmpeg.Logger = Adapt(log)
}

func TestWithGroupSatisfiesFFmpegLogger(t *testing.T) {
	log, err := logger.NewLogger(logger.JsonConfig{Levels: "INFO"})
	if err != nil {
		t.Fatal(err)
	}
	var _ ffmpeg.Logger = WithGroup(log)
}
