package ffmpeg

import (
	"context"
	"log/slog"
	"sync"
	"testing"
)

type captureLogger struct {
	mu   sync.Mutex
	msgs []string
}

func (c *captureLogger) Debug(msg string, args ...any) { c.record("DEBUG", msg) }
func (c *captureLogger) Info(msg string, args ...any)  { c.record("INFO", msg) }
func (c *captureLogger) Warn(msg string, args ...any)  { c.record("WARN", msg) }
func (c *captureLogger) Error(msg string, args ...any) { c.record("ERROR", msg) }

func (c *captureLogger) record(level, msg string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.msgs = append(c.msgs, level+":"+msg)
}

func (c *captureLogger) messages() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.msgs...)
}

func TestNopLoggerDiscardsOutput(t *testing.T) {
	log := NopLogger()
	log.Debug("hidden")
	log.Info("hidden")
	log.Warn("hidden")
	log.Error("hidden")
}

func TestFromSlogNilUsesNop(t *testing.T) {
	log := FromSlog(nil)
	if log == nil {
		t.Fatal("expected non-nil nop logger")
	}
	log.Info("discarded")
}

func TestWithGroupAddsComponentTag(t *testing.T) {
	base := &captureLogger{}
	log := WithGroup(base, "ffmpeg")
	log.Info("detected")

	msgs := base.messages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %v", msgs)
	}
}

func TestConfigLoggerInjection(t *testing.T) {
	capture := &captureLogger{}
	cfg := (&Config{Logger: capture}).withDefaults()
	cfg.Logger.Info("configured")

	msgs := capture.messages()
	if len(msgs) != 1 || msgs[0] != "INFO:configured" {
		t.Fatalf("unexpected messages: %v", msgs)
	}
}

func TestConfigDefaultLoggerUsesSlog(t *testing.T) {
	cfg := (&Config{}).withDefaults()
	if cfg.Logger == nil {
		t.Fatal("expected default logger")
	}
	cfg.Logger.Info("default slog logger")
}

func TestWithGroupOnNilReturnsNop(t *testing.T) {
	log := WithGroup(nil, "ffmpeg")
	if log == nil {
		t.Fatal("expected nop logger")
	}
	log.Info("discarded")
}

func TestServiceLoggerAccessor(t *testing.T) {
	capture := &captureLogger{}
	svc := &Service{cfg: (&Config{Logger: capture}).withDefaults()}
	if svc.Logger() != capture {
		t.Fatal("expected injected logger from Service.Logger()")
	}
}

func TestServiceReloadUsesInjectedLogger(t *testing.T) {
	t.Skip("requires ffmpeg binary; covered by integration tests")

	capture := &captureLogger{}
	ctx := context.Background()
	svc, err := New(ctx, Config{
		FFmpegPath:  "/usr/bin",
		Logger:      capture,
		DetectOnInit: boolPtr(false),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Reload(ctx); err != nil {
		t.Fatal(err)
	}
	if len(capture.messages()) == 0 {
		t.Fatal("expected capability detection logs")
	}
}

func TestFromSlogWrapsSlogLogger(t *testing.T) {
	log := FromSlog(slog.Default())
	if log == nil {
		t.Fatal("expected slog wrapper")
	}
	log.Info("slog wrapper smoke test")
}

func boolPtr(v bool) *bool { return &v }
