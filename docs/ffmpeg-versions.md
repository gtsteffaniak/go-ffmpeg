# FFmpeg versions

## Minimum

Library default: **ffmpeg 5.0.0** (`capabilities.MinSupportedVersion`). Older builds are rejected at detection.

## Version-gated flags

| Feature | Minimum |
|---------|---------|
| `-readrate` | 5.0 |
| Input-side BSF | 7.0 |
| `-readrate_catchup` | 8.0 |

Throttle and HLS pacing omit flags the detected version does not support.

## Builds

- **Full static builds** (e.g. docker `gtstef/ffmpeg:8.1.1`): most encoders and ops enabled.
- **Distro apt ffmpeg**: often stripped (no NVENC, missing libx264). Detection reports what's available; operations gate accordingly.

The library cannot invent encoders not compiled into the binary.

## CI

Integration tests pin a full 8.1.1 image. An apt-ffmpeg job validates detection on stripped builds.
