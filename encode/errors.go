package encode

import (
	"errors"
	"fmt"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
)

var (
	// ErrNotDetected indicates capabilities have not been detected yet.
	ErrNotDetected = errors.New("ffmpeg: capabilities not detected")

	// ErrProfileUnsupported indicates the requested encode/decode profile is unavailable.
	ErrProfileUnsupported = errors.New("ffmpeg: profile unsupported")
)

// ProfileError describes why a VideoProfile or VideoDecodeProfile cannot run.
type ProfileError struct {
	Codec   capabilities.VideoCodec
	Encoder string
	Decoder string
	Accel   capabilities.AccelType
	Reasons []string
}

func (e *ProfileError) Error() string {
	target := e.Encoder
	if target == "" {
		target = e.Decoder
	}
	if target == "" && e.Accel != "" {
		target = string(e.Accel)
	}
	if target == "" {
		target = string(e.Codec)
	}
	return fmt.Sprintf("ffmpeg profile %s unsupported: %v", target, e.Reasons)
}

func (e *ProfileError) Is(target error) bool {
	return target == ErrProfileUnsupported
}
