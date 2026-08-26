# go-ffmpeg

A Go **task wrapper** around FFmpeg and FFprobe. Describe what you want (probe, transcode, HLS segment, screenshot); the library picks encoders, flags, hardware paths, and fallbacks. You focus on the job — not argv archaeology.

**Not** a fluent argv builder or libav/CGO bindings.

## Why go-ffmpeg

Most FFmpeg wrappers still expect you to know demuxers, bitstream filters, rate-control presets, and hardware quirks. go-ffmpeg is a **friendly dev interface** for common media tasks:

1. **Seamless capability support with fallback** — Startup (or lazy) detection learns what the host ffmpeg actually supports. Unsupported codecs, missing HW encoders, or stripped distro builds degrade gracefully instead of failing opaquely at runtime.
2. **Task-optimized flags** — Each operation (`Transcode`, `HLSSegment`, `Screenshot`, …) maps typed options to the right ffmpeg flags for that job. No hand-rolling `-movflags`, `-readrate`, or fMP4 fragment options per call site.
3. **Hardware-aware command selection** — Full encoder/backend detection (NVENC, QSV, VAAPI, VideoToolbox, AMF, …) plus optional smoke tests. The library matches the best encode/decode path for the task and falls back to software when HW is unavailable or unsuitable.
4. **Support matrix, reports, and testing** — `go-ffmpeg` CLI emits human or JSON capability reports. CI runs unit, race, integration, and HLS fixture matrices across software and hardware paths so behavior stays pinned.
5. **Plex-class transcoding and HLS** — Deep support for browser MSE playback: fMP4 segment timelines, `tfdt` alignment, continuous disk-cache HLS, remux/copy/transcode pipelines, and watch-while-transcode pacing — the features streaming servers need, without mastering ffmpeg internals.

## Tasks

- Probe streams, duration, dimensions, subtitles
- Screenshots and MJPEG previews
- Transcode, segmented record, live fMP4 to `io.Writer`
- **HLS** for browser playback (on-demand fMP4 segments + continuous disk cache) — Plex/Jellyfin-class depth
- Timelapse compile
- Startup **capability detection** (encoders, HW backends, enabled operations)

Requires **ffmpeg 5.0+** and **ffprobe** on `PATH` or via config.

## Install

```bash
go get github.com/gtsteffaniak/go-ffmpeg
go install github.com/gtsteffaniak/go-ffmpeg/cmd/go-ffmpeg@latest
```

## Quick start

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
        FFmpegPath: "/usr/local/bin", // directory or binary path
    })
    if err != nil {
        log.Fatal(err)
    }

    if err := svc.Screenshot(ctx, ffmpeg.ScreenshotOptions{
        Input:      ffmpeg.InputSource{URL: "video.mp4", StreamType: ffmpeg.StreamFile},
        OutputPath: "frame.jpg",
        Quality:    85,
    }); err != nil {
        log.Fatal(err)
    }

    fmt.Println("supported:", svc.SupportedOps())
}
```

Set `GPU` in `Config` to enable hardware acceleration (`"dgpu"`, `"igpu"`, render node path, or GPU name). Empty `GPU` means software-only encode.

## Compatibility CLI

Inspect what your host ffmpeg supports before you ship — encoder matrix, HW backends, enabled operations, and version-gated flags:

```bash
go-ffmpeg                    # human-readable capability report
go-ffmpeg -json -o report.json
go-ffmpeg -skip-hw-tests     # CI / headless
```

## Testing

```bash
make test              # unit tests (ffmpeg required on PATH or GOFFMPEG_*_PATH)
make test-race
make test-integration  # requires sample video + ffmpeg
make test-hls          # HLS harness + fixtures (see test/hls/README.md)
make report            # capability report in terminal
```

## Documentation

- API: [pkg.go.dev](https://pkg.go.dev/github.com/gtsteffaniak/go-ffmpeg)
- [docs/operations.md](docs/operations.md) — task-focused API and operation map
- [docs/hls.md](docs/hls.md) — Plex-class on-demand and continuous HLS
- [docs/hardware.md](docs/hardware.md) — detection, backends, and HW fallback
- [docs/ffmpeg-versions.md](docs/ffmpeg-versions.md) — version gates and build matrix
- [docs/security.md](docs/security.md) — caller responsibilities
- [CONTRIBUTING.md](CONTRIBUTING.md) — how to add operations and tests

The HLS benchmark dashboard (`make test-hls` → `make serve-report`) is a **developer harness**, not the public API.

## Security

Caller-controlled URLs and paths are passed to ffmpeg. Validate untrusted input; be aware of SSRF via ffmpeg protocols and concat demuxer risks. The library invokes argv directly (no shell).

## License

MIT — see [LICENSE](LICENSE).
