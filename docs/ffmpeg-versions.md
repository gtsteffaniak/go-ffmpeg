# FFmpeg versions

Different ffmpeg builds ship different encoders and flags. go-ffmpeg gates version-specific options automatically (e.g. `-readrate_catchup` on 8+) and surfaces what's available via capability detection and the `go-ffmpeg` report CLI — so you don't maintain a support matrix by hand.

## Minimum

Library default: **ffmpeg 5.0.0** (`capabilities.MinSupportedVersion`). Older builds are rejected at detection.

`-readrate` has been available since ffmpeg 5.0, so it is always used when throttle is enabled — no separate version gate.

## Version-gated flags

These differ from the library minimum and are omitted on older supported versions:

| Feature | Minimum |
|---------|---------|
| `-readrate_initial_burst` | 6.1 |
| Input-side BSF | 7.0 |
| `-readrate_catchup` | 8.0 |

## Builds

- **Full static builds** (e.g. docker `gtstef/ffmpeg:8.1.1`): most encoders and ops enabled.
- **Distro apt ffmpeg**: often stripped (no NVENC, missing libx264). Detection reports what's available; operations gate accordingly.

The library cannot invent encoders not compiled into the binary.

## CI

Integration tests pin a full 8.1.1 image. An apt-ffmpeg job validates detection on stripped builds.

## Go toolchain

`go.mod` and CI both use **Go 1.25**.
