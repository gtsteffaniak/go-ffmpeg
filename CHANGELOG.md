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
- Frame and sprite **quality** use a unified **1–100** scale (default 85). MJPEG/WebP mapping is shared across `CaptureFrame`, `Screenshot`, and `SpriteSheet`.
- `CaptureFrameFile` publishes outputs with mode **0644** (respecting umask).
- Sprite sheets **pad** each tile to the configured width/height; hardware decode **filter chains** match `VideoDecoderArgs` per backend (VAAPI/D3D12, Vulkan, VideoToolbox, CUDA).
- HLS VOD bounded jobs stop on the **segment boundary** (no `-t` guard); restarts no longer unlink **ready** cached segments or treat `StopIndex` as a partial encode head under explicit-copy mode.

### Migration

- Prefer `CaptureFrame` / `CaptureFrameFile` with `FrameOptions` instead of `Screenshot` / `VideoPreview`. See [docs/capture.md](docs/capture.md).
