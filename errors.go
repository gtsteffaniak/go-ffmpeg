package ffmpeg

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gtsteffaniak/go-ffmpeg/encode"
)

var (
	// ErrUnsupported indicates the requested operation is not supported by the
	// detected FFmpeg build or host platform.
	ErrUnsupported = errors.New("ffmpeg: operation unsupported")

	// ErrBinaryNotFound indicates ffmpeg or ffprobe could not be located.
	ErrBinaryNotFound = errors.New("ffmpeg: binary not found")

	// ErrProbeFailed indicates ffprobe failed to analyze the input.
	ErrProbeFailed = errors.New("ffmpeg: probe failed")

	// ErrEncodeFailed indicates an ffmpeg encode/transcode operation failed.
	ErrEncodeFailed = errors.New("ffmpeg: encode failed")

	// ErrNotDetected indicates capabilities have not been detected yet.
	ErrNotDetected = errors.New("ffmpeg: capabilities not detected")

	// ErrProfileUnsupported indicates the requested encode/decode profile is unavailable.
	ErrProfileUnsupported = errors.New("ffmpeg: profile unsupported")

	// ErrVersionTooOld indicates ffmpeg is below the configured minimum version.
	ErrVersionTooOld = errors.New("ffmpeg: version too old")
)

// FailureKind categorizes ffmpeg stderr for callers (HTTP mapping, retry policy).
type FailureKind = encode.FailureKind

// OperationError wraps an operation failure with context and optional stderr.
type OperationError struct {
	Op     string
	Err    error
	Stderr string
	Kind   FailureKind
}

func (e *OperationError) Error() string {
	if e.Stderr != "" {
		return fmt.Sprintf("ffmpeg %s: %v: %s", e.Op, e.Err, e.Stderr)
	}
	return fmt.Sprintf("ffmpeg %s: %v", e.Op, e.Err)
}

func (e *OperationError) Unwrap() error {
	return e.Err
}

var failureClassifier encode.FailureClassifier

func wrapOp(op string, sentinel error, err error) error {
	return wrapOpWithStderr(op, sentinel, err, "")
}

func wrapOpWithStderr(op string, sentinel error, err error, stderr string) error {
	if err == nil {
		return nil
	}
	if stderr == "" {
		stderr = stderrFromErr(err)
	}
	classified := failureClassifier.Classify(stderr)
	return &OperationError{
		Op:     op,
		Err:    sentinel,
		Stderr: stderr,
		Kind:   classified.Kind,
	}
}

func stderrFromErr(err error) string {
	if err == nil {
		return ""
	}
	var opErr *OperationError
	if errors.As(err, &opErr) && opErr.Stderr != "" {
		return opErr.Stderr
	}
	msg := err.Error()
	if i := strings.LastIndex(msg, ": "); i >= 0 {
		tail := strings.TrimSpace(msg[i+2:])
		if tail != "" && !strings.HasPrefix(tail, "exit status") {
			return tail
		}
	}
	return msg
}

// UnsupportedError describes why an operation is unavailable.
type UnsupportedError struct {
	Op      string
	Reasons []string
}

func (e *UnsupportedError) Error() string {
	return fmt.Sprintf("ffmpeg: %s unsupported: %v", e.Op, e.Reasons)
}

func (e *UnsupportedError) Is(target error) bool {
	return target == ErrUnsupported
}

// VersionTooOldError describes a ffmpeg version below the configured minimum.
type VersionTooOldError struct {
	Version string
	Minimum string
}

func (e *VersionTooOldError) Error() string {
	return fmt.Sprintf("ffmpeg %s is below minimum %s required for transcoding", e.Version, e.Minimum)
}

func (e *VersionTooOldError) Is(target error) bool {
	return target == ErrVersionTooOld
}

// ProfileError describes why a VideoProfile or VideoDecodeProfile cannot run.
type ProfileError = encode.ProfileError
