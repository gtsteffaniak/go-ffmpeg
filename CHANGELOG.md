# Changelog

## v1.6.0 (unreleased)

### Added

- Unified single-frame capture API: `Service.CaptureFrame` (stream to `io.Writer`) and `Service.CaptureFrameFile` (atomic write via temp file + rename).
- `FrameOptions` with `FramePosition` (`AtSec`, `AtPercent`), `CaptureAccuracy` (`CaptureFast`, `CaptureExact`), `FrameFormat` (JPEG, WebP, PNG), scaling, optional `CaptureLightProbe`, and RTSP input options.
- `CaptureFrame` capability in detection; legacy `Screenshot` and `VideoPreview` map to the same requirements.
- Documentation: [docs/capture.md](docs/capture.md) and README frame-capture section.

### Changed

- `Screenshot` and `VideoPreview` are thin wrappers over the capture core (deprecated; behavior preserved, including preview default seek at 10% when unset).
- `VideoPreview` skips a duration ffprobe when `SeekSec` is set and `DurationSec` is unknown.

### Migration

- Prefer `CaptureFrame` / `CaptureFrameFile` with `FrameOptions` instead of `Screenshot` / `VideoPreview`. See [docs/capture.md](docs/capture.md).
