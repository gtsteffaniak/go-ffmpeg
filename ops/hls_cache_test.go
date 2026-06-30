package ops

import (
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/encode"
)

func baseHLSCacheIdentity() HLSCacheIdentity {
	return HLSCacheIdentity{
		SourcePath:         "/media/movie.mkv",
		FileSize:           1_000_000,
		FileModTime:        1_700_000_000_000_000_000,
		Profile:            "quality",
		MaxResolution:      1080,
		MaxBitrate:         0,
		SegmentDurationSec: 4,
		Params: HLSSegmentParams{
			Remux:     false,
			VideoCopy: false,
			MaxHeight: 1080,
			GOP:       120,
			Profile: encode.VideoProfile{
				Codec:   encode.CodecH264,
				Quality: encode.PresetMedium,
				Bitrate: encode.BitrateConfig{Target: "5000k", Max: "7500k"},
			},
		},
	}
}

func TestHLSCacheFingerprintStable(t *testing.T) {
	t.Parallel()
	id := baseHLSCacheIdentity()
	a := HLSCacheFingerprint(id)
	b := HLSCacheFingerprint(id)
	if a == "" || a != b {
		t.Fatalf("fingerprint not stable: %q vs %q", a, b)
	}
	if len(a) != 32 {
		t.Fatalf("fingerprint length = %d, want 32", len(a))
	}
}

func TestHLSCacheFingerprintChangesOnFileStat(t *testing.T) {
	t.Parallel()
	id := baseHLSCacheIdentity()
	base := HLSCacheFingerprint(id)
	id.FileSize++
	if got := HLSCacheFingerprint(id); got == base {
		t.Fatal("expected different fingerprint for file size change")
	}
	id = baseHLSCacheIdentity()
	id.FileModTime++
	if got := HLSCacheFingerprint(id); got == base {
		t.Fatal("expected different fingerprint for mtime change")
	}
}

func TestHLSCacheFingerprintChangesOnProfile(t *testing.T) {
	t.Parallel()
	id := baseHLSCacheIdentity()
	base := HLSCacheFingerprint(id)
	id.Profile = "balanced"
	if got := HLSCacheFingerprint(id); got == base {
		t.Fatal("expected different fingerprint for profile change")
	}
}

func TestHLSCacheFingerprintChangesOnEncodePath(t *testing.T) {
	t.Parallel()
	id := baseHLSCacheIdentity()
	base := HLSCacheFingerprint(id)
	id.Params.VideoCopy = true
	if got := HLSCacheFingerprint(id); got == base {
		t.Fatal("expected different fingerprint for video copy path")
	}
	id = baseHLSCacheIdentity()
	id.Params.GOP = 60
	if got := HLSCacheFingerprint(id); got == base {
		t.Fatal("expected different fingerprint for GOP change")
	}
}

func TestHLSCacheSchemaVersion(t *testing.T) {
	t.Parallel()
	if HLSCacheSchemaVersion() != "v1" {
		t.Fatalf("schema = %q", HLSCacheSchemaVersion())
	}
}
