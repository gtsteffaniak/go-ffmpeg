package encode

import (
	"strings"
)

// FailureKind categorizes ffmpeg stderr for callers (HTTP mapping, retry policy).
type FailureKind string

const (
	FailureUnknown        FailureKind = "unknown"
	FailureInputNotFound  FailureKind = "input_not_found"
	FailureInputInvalid   FailureKind = "input_invalid"
	FailurePermission     FailureKind = "permission"
	FailureEncoder        FailureKind = "encoder"
	FailureDecoder        FailureKind = "decoder"
	FailureHardware       FailureKind = "hardware"
	FailureTimeout        FailureKind = "timeout"
	FailureOutputEmpty    FailureKind = "output_empty"
	FailureEncode         FailureKind = "encode"
	FailureSeek           FailureKind = "seek"
)

// ClassifiedFailure is the result of parsing ffmpeg stderr.
type ClassifiedFailure struct {
	Kind    FailureKind
	Message string
}

// FailureClassifier parses ffmpeg stderr for common failure patterns.
type FailureClassifier struct{}

// Classify inspects stderr (and optional exit message) and returns a failure kind.
func (FailureClassifier) Classify(stderr string) ClassifiedFailure {
	lower := strings.ToLower(stderr)
	if lower == "" {
		return ClassifiedFailure{Kind: FailureUnknown}
	}

	match := func(substr string) bool {
		return strings.Contains(lower, substr)
	}

	switch {
	case match("no such file or directory"):
		return ClassifiedFailure{Kind: FailureInputNotFound, Message: "input file not found"}
	case match("permission denied"):
		return ClassifiedFailure{Kind: FailurePermission, Message: "permission denied"}
	case match("invalid data found when processing input"):
		return ClassifiedFailure{Kind: FailureInputInvalid, Message: "invalid or corrupt input"}
	case match("could not find codec") || match("unknown encoder") || match("encoder") && match("not found"):
		return ClassifiedFailure{Kind: FailureEncoder, Message: "encoder not available"}
	case match("could not open decoder") || match("decoder") && match("not found"):
		return ClassifiedFailure{Kind: FailureDecoder, Message: "decoder not available"}
	case match("cannot create cuda") || match("cannot load nvidia") ||
		match("no device available") || match("device setup failed") ||
		match("failed to initialise") && match("vaapi") || match("mfx") && match("error"):
		return ClassifiedFailure{Kind: FailureHardware, Message: "hardware acceleration failed"}
	case match("timed out") || match("timeout"):
		return ClassifiedFailure{Kind: FailureTimeout, Message: "operation timed out"}
	case match("output file is empty") || match("nothing was encoded"):
		return ClassifiedFailure{Kind: FailureOutputEmpty, Message: "ffmpeg produced empty output"}
	case match("seek") && (match("failed") || match("impossible")):
		return ClassifiedFailure{Kind: FailureSeek, Message: "seek failed"}
	case match("conversion failed"):
		return ClassifiedFailure{Kind: FailureEncode, Message: "conversion failed"}
	case match("error while opening input"):
		return ClassifiedFailure{Kind: FailureInputInvalid, Message: "could not open input"}
	default:
		return ClassifiedFailure{Kind: FailureUnknown, Message: strings.TrimSpace(stderr)}
	}
}
