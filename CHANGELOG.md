# Changelog

## v1.6.0 (unreleased)

### Added

- Unified single-frame capture API: `Service.CaptureFrame` (stream to `io.Writer`) and `Service.CaptureFrameFile` (atomic write via temp file + rename).
- `FrameOptions` with `FramePosition` (`AtSec`, `AtPercent`), `CaptureAccuracy` (`CaptureFast`, `CaptureExact`), `FrameFormat` (JPEG, WebP, PNG), scaling, optional `CaptureLightProbe`, and RTSP input options.
- `CaptureFrame` capability in detection; legacy `Screenshot` and `VideoPreview` map to the same requirements.
- Documentation: [docs/capture.md](docs/capture.md) and README frame-capture section.

### Changed

- `Screenshot` and `VideoPreview` are thin wrappers over the capture core (deprecated; behavior preserved, including preview default seek at 10% when unset). Deprecated operations are no longer advertised: `ops` exposes `Operation.Deprecated()` / `ops.IsDeprecated`, `EvaluateOps` keeps them out of `EnabledOps`/`DisabledOps`, and `Capabilities.DeprecatedOps` + the capability report list them separately. They remain callable.
- Startup hardware warnings are de-duplicated through one source (`Capabilities.StartupHardwareWarnLines`): one `hardware probe:` line per distinct remediation and one `vulkan probe:` line naming every affected Vulkan encoder. `GPUPresentRemediationLogLines()` / `VulkanProbeWarnLines()` are now filtered views of the same source and can no longer duplicate text.
- Probe classification no longer promotes hardware capability limits or driver feature gaps to `gpu_present_driver_missing`. AV1 (and other missing NVENC codecs, e.g. Ampere) classify as `nvenc_unsupported_gpu`; Vulkan encode failures, including ENOSYS / `-38 Function not implemented` and `VK_KHR_video_encode_queue` gaps, classify as `vulkan_no_video_encode_driver`. WSL remediation is centralized and WSL-aware: it reports the missing NVIDIA Vulkan ICD and points at NVENC instead of a `/dev/dri` render node or a driver update.
- `VideoPreview` skips a duration ffprobe when `SeekSec` is set and `DurationSec` is unknown.
- Frame and sprite **quality** use a unified **1–100** scale (default 85). MJPEG/WebP mapping is shared across `CaptureFrame`, `Screenshot`, and `SpriteSheet`.
- `CaptureFrameFile` publishes outputs with explicit mode **0644**.
- Sprite sheets **pad** each tile to the configured width/height; hardware decode **filter chains** match `VideoDecoderArgs` per backend (VAAPI/D3D12, Vulkan, VideoToolbox, CUDA).
- HLS VOD bounded jobs stop on the **segment boundary** (no `-t` guard); restarts no longer unlink **ready** cached segments or treat `StopIndex` as a partial encode head under explicit-copy mode.

### Migration

- Prefer `CaptureFrame` / `CaptureFrameFile` with `FrameOptions` instead of `Screenshot` / `VideoPreview`. See [docs/capture.md](docs/capture.md).
