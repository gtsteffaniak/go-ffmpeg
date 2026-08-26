# go-ffmpeg

A Go **task wrapper** around FFmpeg and FFprobe. Pass configurations (`VideoProfile`, probe options, HLS settings); the library owns ffmpeg flags, codec selection, hardware fallback, and version-safe options.

**Not** a fluent argv builder, not libav/CGO bindings, not HLS-only.

## Tasks

- Probe streams, duration, dimensions, subtitles
- Screenshots and MJPEG previews
- Convert (HEIC → JPEG; more formats over time)
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

```bash
go-ffmpeg                    # human-readable capability report
go-ffmpeg -json -o report.json
go-ffmpeg -skip-hw-tests     # CI / headless
```

## Testing

```bash
make test              # unit tests (skips if ffmpeg missing unless GOFFMPEG_REQUIRE_FFMPEG=1)
make test-race
make test-integration  # requires sample video + ffmpeg
make test-hls          # HLS harness + fixtures (see test/hls/README.md)
make report            # capability report in terminal
```

## Documentation

- API: [pkg.go.dev](https://pkg.go.dev/github.com/gtsteffaniak/go-ffmpeg)
- [docs/operations.md](docs/operations.md) — task map
- [docs/hls.md](docs/hls.md) — on-demand vs continuous HLS
- [docs/hardware.md](docs/hardware.md) — GPU / backends
- [docs/ffmpeg-versions.md](docs/ffmpeg-versions.md) — version flags
- [docs/security.md](docs/security.md) — caller responsibilities
- [CONTRIBUTING.md](CONTRIBUTING.md) — how to add operations and tests

The HLS benchmark dashboard (`make test-hls` → `make serve-report`) is a **developer harness**, not the public API.

## Security

Caller-controlled URLs and paths are passed to ffmpeg. Validate untrusted input; be aware of SSRF via ffmpeg protocols and concat demuxer risks. The library invokes argv directly (no shell).

## License

MIT — see [LICENSE](LICENSE).
