# go-ffmpeg

A Go wrapper library **and CLI** for FFmpeg and FFprobe with startup capability detection, config-driven operations, and a long-lived `Service` for fast runtime use.

## Features

- **Capability detection** on startup: binary version, build flags, encoders/decoders, filters, protocols, platform GPU gates, and optional hardware encoder smoke tests
- **Compatibility CLI** (`go-ffmpeg`) — run a full report on any system without writing code
- **Human-readable report** plus JSON export of the full capability matrix
- **Pluggable operations** with pre-flight checks (`ProbeStream`, `Screenshot`, `Transcode`, and more)
- **Config-based encoding** via `VideoProfile` and automatic encoder resolution (NVENC → AMF → QSV → VAAPI → software)
- **Concurrency control** via service semaphore
- **Integration tests** compatible with [gtstef/ffmpeg:8.1.1](https://hub.docker.com/r/gtstef/ffmpeg)

## Install

**Library:**

```bash
go get github.com/gtsteffaniak/go-ffmpeg
```

**CLI binary:**

```bash
go install github.com/gtsteffaniak/go-ffmpeg/cmd/go-ffmpeg@latest
```

Or build from source:

```bash
make build
./bin/go-ffmpeg
make report   # prints report to console and saves compatibility-report.txt
```

Requires `ffmpeg` and `ffprobe` on the host or a custom path.

## Compatibility CLI

Run a full capability report for the FFmpeg installation on the current system:

```bash
go-ffmpeg
```

Use a specific binary or directory:

```bash
go-ffmpeg -ffmpeg-path /usr/local/bin/ffmpeg
go-ffmpeg -ffmpeg-path /opt/custom/bin -ffprobe-path /opt/custom/bin/ffprobe
```

JSON output (for scripts, CI, or storage):

```bash
go-ffmpeg -json
go-ffmpeg -json -o compatibility.json
```

Skip slow hardware encoder smoke tests (useful in CI or headless containers):

```bash
go-ffmpeg -skip-hw-tests
```

By default the CLI probes every hardware backend on the system. Use `-skip-hw-tests` only for CI or headless environments.

The report is structured as:

1. **FFmpeg build** — configure flags, compiled libraries, hwaccels, filters, protocols
2. **System platform** — detected GPUs and driver gates
3. **Selected GPU** — device, vendor, render node, encoder hierarchy (filebrowser `gpu` config only)
4. **Hardware backends** — Software, NVENC, QSV, VAAPI, AMF, VideoToolbox sections with per-codec compile + runtime results
5. **Codec resolution** — preferred encode/decode path for the active scope
6. **Operations** — enabled/disabled library operations

Color output (auto-detected when stdout is a TTY):

```bash
go-ffmpeg -color always
make report   # colored console output + saved report file
```

Environment variables (used when flags are omitted):

| Variable | Purpose |
|----------|---------|
| `GOFFMPEG_FFMPEG_PATH` | Default `-ffmpeg-path` |
| `GOFFMPEG_FFPROBE_PATH` | Default `-ffprobe-path` |
| `GOFFMPEG_SKIP_HW` | Set to `1` to skip HW tests |

Example report excerpt:

```
=== go-ffmpeg capability report ===
Binary: ffmpeg 8.1.1 @ /usr/local/bin/ffmpeg
Build profile: full
Platform: linux/amd64 | NVIDIA: false | DRI: true | QSV: true | VAAPI: true
---
Codec resolution:
  h264 -> libx264 (none)
  av1 -> libsvtav1 (none)
---
Operations enabled: ProbeStream, GetMediaDuration, Screenshot, Transcode, ...
```

## Quick start (library)

```go
package main

import (
    "context"
    "fmt"
    "log"

    ffmpeg "github.com/gtsteffaniak/go-ffmpeg"
)

func main() {
    ctx := context.Background()
    svc, err := ffmpeg.New(ctx, ffmpeg.Config{
        FFmpegPath: "/usr/local/bin", // directory or full binary path
    })
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println(svc.Capabilities().ReportString())

    info, err := svc.ProbeStream(ctx, ffmpeg.ProbeStreamOptions{
        URL:        "rtsp://camera/stream",
        StreamType: ffmpeg.StreamRTSP,
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("video=%s audio=%s\n", info.VideoCodec, info.AudioCodec)
}
```

## Docker note

The `gtstef/ffmpeg` image uses `ENTRYPOINT ["/ffmpeg"]`. To check the version:

```bash
docker run --rm --entrypoint /ffmpeg gtstef/ffmpeg:8.1.1 -version
```

Do **not** run `docker run gtstef/ffmpeg:8.1.1 ffmpeg version` — the word `ffmpeg` is treated as an output filename.

Extract binaries for local/CI use:

```bash
id=$(docker create gtstef/ffmpeg:8.1.1)
docker cp "$id:/ffmpeg" ./ffmpeg
docker cp "$id:/ffprobe" ./ffprobe
docker rm "$id"
chmod +x ./ffmpeg ./ffprobe
export GOFFMPEG_FFMPEG_PATH=$PWD/ffmpeg
export GOFFMPEG_FFPROBE_PATH=$PWD/ffprobe
```

## Operations

| Method | Description |
|--------|-------------|
| `ProbeStream` | Validate and probe RTSP/HLS/file streams |
| `GetMediaDuration` | Read duration via ffprobe |
| `GetImageDimensions` | Read width/height |
| `Screenshot` | Extract a single JPEG/PNG frame |
| `VideoPreview` | MJPEG preview frame to `io.Writer` |
| `Transcode` | Re-encode with `VideoProfile` |
| `SegmentRecord` | Segmented MP4 recording |
| `FMP4StreamCopy` | Live fragmented MP4 to stdout |
| `TimelapseCompile` | Build video from concat list |
| `DetectSubtitles` / `ExtractSubtitle` | Subtitle probe and WebVTT extract |
| `ConvertHEIC` | HEIC/HEIF to JPEG |

Unsupported operations return `ffmpeg.ErrUnsupported` with reasons from the capability matrix.

Unsupported encode/decode profiles return `ffmpeg.ErrProfileUnsupported` (`ProfileError`) when validation fails before ffmpeg runs.

## Encoding and decode selection

On startup, `Service` caches the full capability matrix. By default, operations pick the best hardware path (NVENC → AMF → QSV → VAAPI → software). Configure GPU selection on the service (hardware acceleration is disabled when `gpu` is empty):

```go
svc, _ := ffmpeg.New(ctx, ffmpeg.Config{
    GPU: "igpu", // or "dgpu", "/dev/dri/renderD129", "GeForce RTX 4090"
})
```

Callers can still override per request:

```go
// Automatic — uses detected preferred encoder (e.g. h264_qsv on Intel)
profile := ffmpeg.VideoProfile{Codec: ffmpeg.CodecH264}

// Force software
profile := ffmpeg.VideoProfile{Codec: ffmpeg.CodecH264, ForceSoftware: true}
// or
profile := ffmpeg.VideoProfile{Codec: ffmpeg.CodecH264, Accel: ffmpeg.AccelNone}

// Force a hardware backend
profile := ffmpeg.VideoProfile{Codec: ffmpeg.CodecH264, Accel: ffmpeg.AccelQSV}

// Force a specific ffmpeg encoder
profile := ffmpeg.VideoProfile{Codec: ffmpeg.CodecAV1, Encoder: "libsvtav1"}
```

List cached options (no re-probing):

```go
for _, opt := range svc.AvailableEncodeOptions() {
    fmt.Println(opt.Codec, opt.Encoder, opt.Accel, opt.Label)
}

if err := svc.ValidateVideoProfile(profile); err != nil {
    // errors.Is(err, ffmpeg.ErrProfileUnsupported)
    log.Fatal(err)
}
```

`ResolveVideoEncoder` / `ResolveVideoDecoder` return the selection without building ffmpeg arguments. Decode overrides use the same pattern on `VideoDecodeProfile`.

## Testing

Two modes:

```bash
# Unit tests (fast)
make test

# Integration tests: ffmpeg, HLS matrix, writes test/hls/report_site/data/report.json
make integration-tests
make serve-results       # http://127.0.0.1:8765/ — browse the dashboard
```

`make report` is separate — the ffmpeg **capability report** in the terminal, not the integration dashboard.

The HLS harness lives in `test/hls/`. It uses the bundled sample at `test/data/Big_Buck_Bunny_1080_10s_2MB.mp4` by default. See [test/hls/README.md](test/hls/README.md) for CLI details.

## Configuration

```go
svc, err := ffmpeg.New(ctx, ffmpeg.Config{
    FFmpegPath:     "/opt/ffmpeg",
    FFprobePath:    "/opt/ffprobe",
    DetectOnInit:   ptr(true),
    DetectTimeout:  60 * time.Second,
    MaxConcurrent:  4,
    SkipHWTests:    false,
    EncoderHierarchy: []capabilities.AccelType{capabilities.AccelNVENC, capabilities.AccelQSV},
})
```

## Logging

go-ffmpeg uses **dependency injection** for logging, similar to [go-logger](https://github.com/gtsteffaniak/go-logger). Pass a logger through `Config.Logger`; the library never relies on global log state.

### Recommended: inject go-logger

```go
import (
    ffmpeg "github.com/gtsteffaniak/go-ffmpeg"
    "github.com/gtsteffaniak/go-ffmpeg/gtlogger"
    "github.com/gtsteffaniak/go-logger/logger"
)

log, err := logger.NewLogger(logger.JsonConfig{Levels: "INFO,DEBUG"})
if err != nil {
    panic(err)
}

svc, err := ffmpeg.New(ctx, ffmpeg.Config{
    FFmpegPath: "/usr/local/bin",
    Logger:     gtlogger.WithGroup(log), // tags output with group=ffmpeg
})
```

Any `logger.Logger` instance works directly — `gtlogger.Adapt(log)` is optional sugar.

### Use slog or silence detection logs

```go
import "log/slog"

// slog.Default() wrapper
svc, err := ffmpeg.New(ctx, ffmpeg.Config{
    Logger: ffmpeg.FromSlog(slog.Default()),
})

// CLI-style: no detection chatter (go-ffmpeg binary uses this)
svc, err := ffmpeg.New(ctx, ffmpeg.Config{
    Logger: ffmpeg.NopLogger(),
})
```

### Logger interface

Libraries should accept `ffmpeg.Logger` (four structured methods: Debug, Info, Warn, Error). `Service.Logger()` returns the configured instance for downstream use.

```go
type Logger interface {
    Debug(msg string, args ...any)
    Info(msg string, args ...any)
    Warn(msg string, args ...any)
    Error(msg string, args ...any)
}
```

During capability detection, logs are emitted under the `ffmpeg` group when the underlying logger supports grouping.

## License

See repository license.
