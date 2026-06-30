package ops

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/gtsteffaniak/go-ffmpeg/encode"
)

const hlsCacheSchemaVersion = "v1"

// HLSCacheIdentity describes everything that affects on-demand HLS segment bytes.
type HLSCacheIdentity struct {
	SourcePath         string
	FileSize           int64
	FileModTime        int64 // unix nanoseconds
	Profile            string
	MaxResolution      int
	MaxBitrate         int
	SegmentDurationSec float64
	Params             HLSSegmentParams
}

// HLSCacheFingerprint returns a stable directory name for a transcode cache entry.
func HLSCacheFingerprint(id HLSCacheIdentity) string {
	var b strings.Builder
	b.WriteString(hlsCacheSchemaVersion)
	b.WriteByte('\n')
	b.WriteString(id.SourcePath)
	b.WriteByte('\n')
	fmt.Fprintf(&b, "size=%d\n", id.FileSize)
	fmt.Fprintf(&b, "mtime=%d\n", id.FileModTime)
	b.WriteString(id.Profile)
	b.WriteByte('\n')
	fmt.Fprintf(&b, "maxRes=%d\n", id.MaxResolution)
	fmt.Fprintf(&b, "maxBr=%d\n", id.MaxBitrate)
	fmt.Fprintf(&b, "segDur=%.6f\n", id.SegmentDurationSec)
	appendHLSSegmentParams(&b, id.Params)
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:16])
}

// HLSCacheSchemaVersion returns the active fingerprint schema version.
func HLSCacheSchemaVersion() string {
	return hlsCacheSchemaVersion
}

func appendHLSSegmentParams(b *strings.Builder, p HLSSegmentParams) {
	fmt.Fprintf(b, "remux=%t\n", p.Remux)
	fmt.Fprintf(b, "videoCopy=%t\n", p.VideoCopy)
	fmt.Fprintf(b, "maxH=%d\n", p.MaxHeight)
	fmt.Fprintf(b, "gop=%d\n", p.GOP)
	appendVideoDecodeProfile(b, p.Decode)
	appendVideoProfile(b, p.Profile)
}

func appendVideoDecodeProfile(b *strings.Builder, p encode.VideoDecodeProfile) {
	fmt.Fprintf(b, "decCodec=%s\n", p.Codec)
	fmt.Fprintf(b, "decAccel=%s\n", p.Accel)
	fmt.Fprintf(b, "decDecoder=%s\n", p.Decoder)
	fmt.Fprintf(b, "decSw=%t\n", p.ForceSoftware)
}

func appendVideoProfile(b *strings.Builder, p encode.VideoProfile) {
	fmt.Fprintf(b, "encCodec=%s\n", p.Codec)
	fmt.Fprintf(b, "encQuality=%s\n", p.Quality)
	fmt.Fprintf(b, "encTarget=%s\n", p.Bitrate.Target)
	fmt.Fprintf(b, "encMin=%s\n", p.Bitrate.Min)
	fmt.Fprintf(b, "encMax=%s\n", p.Bitrate.Max)
	fmt.Fprintf(b, "encBuf=%s\n", p.Bitrate.BufSize)
	fmt.Fprintf(b, "encGop=%d\n", p.GOP)
	fmt.Fprintf(b, "encAccel=%s\n", p.Accel)
	fmt.Fprintf(b, "encEncoder=%s\n", p.Encoder)
	fmt.Fprintf(b, "encSw=%t\n", p.ForceSoftware)
}
