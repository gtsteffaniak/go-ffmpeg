# go-ffmpeg

A Go wrapper library **and CLI** for FFmpeg and FFprobe with startup capability detection, config-driven operations, and a long-lived `Service` for fast runtime use.

## Features

- **Capability detection** on startup: binary version, build flags, encoders/decoders, filters, protocols, platform GPU gates, and optional hardware encoder smoke tests
- **Compatibility CLI** (`go-ffmpeg`) — run a full report on any system without writing code
- **Human-readable report** plus JSON export of the full capability matrix
- **Pluggable operations** with pre-flight checks — probe, transcode, HLS (on-demand and continuous), fMP4 streaming, subtitles, images, and more
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
Operations enabled: ProbeStream, GetMediaDuration, GetImageDimensions, Screenshot, VideoPreview, Transcode, SegmentRecord, FMP4StreamCopy, FMP4Transcode, HLSSegment, TimelapseCompile, ExtractSubtitle, ConvertHEIC, DetectSubtitles
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

Capability detection gates each operation at startup. `Service.SupportedOps()` lists what the current FFmpeg build and platform can run. Unsupported operations return `ffmpeg.ErrUnsupported` with reasons from the capability matrix. Unsupported encode/decode profiles return `ffmpeg.ErrProfileUnsupported` (`ProfileError`) when validation fails before ffmpeg runs.

### Service

| Method | Description |
|--------|-------------|
| `New` / `Reload` | Create service and run (or refresh) capability detection |
| `Capabilities` | Full capability matrix (`ReportString`, JSON export) |
| `SupportedOps` | Enabled operation names from detection |
| `Acquire` / `Release` | Concurrency semaphore (respects `MaxConcurrent`) |
| `FFmpegPath` / `FFprobePath` | Resolved binary paths |
| `Logger` | Configured `Logger` instance |


| Method | Description |
|--------|-------------|
| `ProbeStream` | Validate and probe RTSP/HLS/file streams |
| `GetMediaDuration` | Read duration via ffprobe |
| `GetImageDimensions` | Read width/height of an image file |
| `Screenshot` | Extract a single JPEG/PNG frame |
| `VideoPreview` | MJPEG preview frame to `io.Writer` |
| `Transcode` | Re-encode with `VideoProfile` |
| `SegmentRecord` | Segmented MP4 recording |
| `FMP4StreamCopy` | Live fragmented MP4 stream copy to `io.Writer` |
| `FMP4Transcode` | Live fragmented MP4 transcode to `io.Writer` |
| `HLSSegment` | On-demand fMP4 HLS media segment (remux, copy, or transcode) |
| `TimelapseCompile` | Build video from concat list |
| `DetectSubtitles` | List embedded subtitle tracks |
| `ExtractSubtitle` | Extract a subtitle stream to WebVTT |
| `ConvertHEIC` | HEIC/HEIF to JPEG |

### HLS — on-demand segments

Per-segment encoding for browser MSE playback (independent `ffmpeg` invocations per segment index).

| Method / helper | Description |
|-----------------|-------------|
| `HLSSegment` | Encode one fMP4 media fragment (`moof`+`mdat`) |
| `HLSInitAndSegment` | Init segment + media fragment in one call |
| `HLSSegmentMedia` | Stream media fragment bytes to `io.Writer` |
| `BuildHLSSegmentParams` | Resolve remux/copy/transcode params for a file |
| `DescribeHLSSegmentPlan` | Human-readable summary of resolved HLS params |
| `BuildHLSSegmentOptions` | Build `HLSSegmentOptions` for segment index *n* |
| `BuildHLSSegmentBuildInput` | Derive remux/copy/transcode flags from stream info |
| `BuildHLSSegmentParamsFast` | Assemble params without probing fps |
| `BuildHLSSegmentTimeline` | Keyframe-aligned segment start times and durations |
| `SanitizeHLSKeyframes` | Filter spurious keyframe probe times |
| `KeyframeSeekBefore` | Largest keyframe time ≤ *sec* |
| `NeedsFullVideoTranscode` | Whether video must be re-encoded |
| `UseVideoCopy` | H.264 stream-copy with audio transcode |
| `CanFMP4StreamCopy` | Whether remux to fMP4 is possible |
| `CanH264VideoCopy` | H.264 copy with audio transcode |
| `HLSSegmentGOP` | GOP size from fps and on-demand defaults |
| `DefaultOnDemandHLSDefaults` | Default segment duration and GOP settings |
| `HLSDecodeProfileForOnDemand` | Input decode profile for HLS transcode |
| `DefaultHLSVideoProfile` | Safe H.264 transcode defaults |

### HLS — continuous jobs

Long-running `ffmpeg -f hls` writing `init.m4s` and `seg/%05d.m4s` under a cache directory (same pipeline as filebrowser disk cache).

| Method / type | Description |
|---------------|-------------|
| `StartHLSContinuous` | Launch continuous HLS job; returns `*HLSContinuousJob` |
| `HLSContinuousJob.Wait` | Block until ffmpeg exits and segment alignment completes |
| `HLSContinuousJob.Cancel` | Stop the ffmpeg process |
| `HLSContinuousJob.Done` | Channel that fires when ffmpeg exits (before alignment) |
| `HLSContinuousJob.VTDecodeUnreliable` | VideoToolbox decode failures detected in stderr |
| `ops.AlignContinuousHLSSegments` | Rebase all segment `tfdt` to the HLS media timeline |
| `ops.AlignContinuousSegmentFile` | Rebase one segment file to a target media start |
| `ops.ValidateContinuousHLSOutput` | Validate playlist + per-segment timeline |
| `ops.FixContinuousPlaylistTargetDuration` | Set `#EXT-X-TARGETDURATION` from max `#EXTINF` |
| `ops.FixContinuousPlaylistSegmentURIs` | Rewrite bare filenames to `seg/NNNNN.m4s` |

**Input pacing** (`HLSContinuousPacing` on `HLSContinuousOptions`; overridden when `Throttle.Enabled` is true):

| Pacing | Use case | ffmpeg behavior |
|--------|----------|-----------------|
| `HLSContinuousCacheFill` (default) | Background disk cache prefetch | No readrate — encode at max CPU/GPU speed |
| `HLSContinuousLivePaced` | Watch-while-transcode | `-readrate 1`, `-readrate_catchup 2`, `-readrate_initial_burst 60` (tunable via `ThrottleConfigLivePaced`) |
| `HLSContinuousRemuxSegmentDeletion` | Stream-copy + segment deletion | `-readrate 10`, high catchup (Jellyfin-style) |

```go
// Background cache — fill disk as fast as possible
job, err := svc.StartHLSContinuous(ctx, goffmpeg.HLSContinuousOptions{
    OutputDir: cacheDir,
    Pacing:    goffmpeg.HLSContinuousCacheFill, // default zero value
    ...
})

// Active playback — pace at ~1x after a 60s startup burst
job, err := svc.StartHLSContinuous(ctx, goffmpeg.HLSContinuousOptions{
    OutputDir: cacheDir,
    Pacing:    goffmpeg.HLSContinuousLivePaced,
    ...
})

// Or set throttle explicitly (overrides Pacing):
Throttle: encode.ThrottleConfigLivePaced(90),
```

Head start for disk cache comes from **max-speed encode** plus serving segments when `seg/NNNNN.m4s.ready` exists. For live sessions, pair `HLSContinuousLivePaced` with **app-level pause** when segments are farther ahead of the player than your buffer setting (Jellyfin/Plex model).

Continuous jobs support remux, video-copy, and full transcode; resume via `StartIndex` / `StartSec`.

### HLS — disk cache

| Method / helper | Description |
|-----------------|-------------|
| `HLSCacheFingerprint` | Stable SHA-based directory name for a cache entry |
| `HLSCacheIdentity` | Inputs that affect cached segment bytes (path, mtime, profile, encode params) |
| `HLSCacheSchemaVersion` | Active fingerprint schema version (`v1`) |

### Probe helpers

| Method | Description |
|--------|-------------|
| `ProbeFile` | Probe a local media file (`ProbeStream` with `StreamFile`) |
| `ProbeVideoFPS` | Video frame rate via ffprobe |
| `ProbeVideoKeyframeTimes` | Keyframe timestamps for HLS segment planning |

### Encode / decode resolution

| Method | Description |
|--------|-------------|
| `VideoEncoderArgs` | ffmpeg video encoder arguments for a `VideoProfile` |
| `VideoDecoderArgs` | ffmpeg video decoder arguments for a `VideoDecodeProfile` |
| `ValidateVideoProfile` | Pre-flight encode profile check |
| `ResolveVideoEncoder` | Encoder selection without building full args |
| `ValidateVideoDecodeProfile` | Pre-flight decode profile check |
| `ResolveVideoDecoder` | Decoder selection without building full args |
| `EncodeOptions` / `AvailableEncodeOptions` | Cached encode paths from detection |
| `DecodeOptions` / `AvailableDecodeOptions` | Cached decode paths from detection |

### Throttle presets (`encode` package)

| Function | Description |
|----------|-------------|
| `ThrottleConfigOff` | Max encode speed (background disk cache) |
| `ThrottleConfigLivePaced` | `-readrate 1` + catchup + `initial_burst` for watch-while-transcode |
| `ThrottleConfigRemuxSegmentDeletion` | Jellyfin-style `-readrate 10` for stream-copy + segment deletion |
| `DefaultLivePacedBurstSec` | Default burst seconds (60) for live-paced jobs |

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

```bash
make test              # unit tests (fast)
make test-integration  # go integration tests (ffmpeg required)
make test-hls          # all fixtures + benchmarks → test/hls/report_site/
make serve-report      # browse dashboard at http://127.0.0.1:8765/
make lint
make format
```

`make report` is separate — the ffmpeg **capability report** in the terminal, not the HLS dashboard.

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
