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

## Supported matrix versions

Pinned in [`docker/versions`](../docker/versions) (regular `gtstef/ffmpeg` images, not `-decode`):

| Version | Image |
|---------|-------|
| 5.1.9 | `gtstef/ffmpeg:5.1.9` |
| 6.1.6 | `gtstef/ffmpeg:6.1.6` |
| 7.1.5 | `gtstef/ffmpeg:7.1.5` |
| 8.1.2 | `gtstef/ffmpeg:8.1.2` |
| 9.0.1 | `gtstef/ffmpeg:9.0.1` |

Tests run **inside** `docker/test.Dockerfile` containers (`golang:1.25-bookworm` + `COPY --from=gtstef/ffmpeg:VERSION /ffmpeg /ffprobe`), matching the filebrowser image pattern.

## Builds

- **Full static builds** (`gtstef/ffmpeg:X.Y.Z`): most encoders and ops enabled.
- **Distro apt ffmpeg**: often stripped (no NVENC, missing libx264). Detection reports what's available; operations gate accordingly.

The library cannot invent encoders not compiled into the binary.

## CI and local matrix

- **`ffmpeg-matrix`:** one job per version in `docker/versions` — detection, version-gated throttle tests, integration subset, CLI report (all in docker).
- **Integration / HLS:** default `gtstef/ffmpeg:8.1.2` via `scripts/docker/run-integration-tests.sh` and `run-hls-tests.sh`.
- **Pre-push hook:** `make test-ffmpeg-matrix` (parallel, same gates as CI matrix).
- **`apt-ffmpeg`:** stripped distro detection on the host runner.

Set `GOFFMPEG_MATRIX_SKIP_MISSING=1` locally to skip versions whose docker image is not published yet.

## Go toolchain

`go.mod` and CI both use **Go 1.25**.
