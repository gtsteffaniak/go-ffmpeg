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
- **Linux transcode fix:** with hardware decode, the VAAPI/D3D12 and NVENC encode filter graphs no longer feed GPU surfaces (`-hwaccel_output_format vaapi|cuda`) into software `scale`/`format` filters, which failed with `Impossible to convert between the formats ... src: vaapi|cuda` (HLS sessions then 503ed until the client timed out). VAAPI now uses `scale_vaapi` (also converting 10-bit to nv12 on the GPU), or `hwdownload,format=nv12|p010le` when it is missing. NVENC uses `scale_cuda=...:format=nv12,hwdownload,format=nv12` (software nv12 frames reach the encoder, so `-pix_fmt yuv420p` stays valid); without `scale_cuda` the CUDA decoder args no longer set `-hwaccel_output_format cuda`, so frames reach the plain software graph in system memory. Sprite sheets follow the same rule. `scale_vaapi`, `scale_cuda` and `scale_qsv` are now probed as known filters. VAAPI hw decode no longer re-inits the `va` device in the filter args.
- New probe code `hw_codec_unsupported`: VAAPI `No usable encoding entrypoint` and QSV `Current codec type is unsupported` are hardware limits. QSV parameter-only rejections (for example AV1 on Intel Gen12) stay `unknown`, and are demoted to `hw_codec_unsupported` only when the classifier could not explain them (`EncoderProbeResult.OriginalCode == unknown`) and another encoder of the same backend passed its probe; genuine driver-init/permission/library failures are never demoted, so a working Intel iGPU no longer gets told to `pacman -S intel-media-driver`.
- Hardware capability limits are logged once at **info** via `Capabilities.StartupHardwareInfoLines()` (no install advice) instead of one warning per encoder. Probe `Detail` is now a single compact line (no `[x @ 0x...]` prefixes, no noise lines, cut at a word boundary), and the leaked "Not an OS or FFmpeg compile issue" hints were removed.
- `Service.Reload` now logs one hardware overview line from `Capabilities.HardwareSupportSummary()`, for example `hardware encode: VAAPI [h264 hevc], QSV [h264 hevc] | decode: h264 hevc av1 vp9 | no hw encode: av1 vp9 (GPU limit)`. It is `info` normally and `warn` only when support is incomplete for a driver-fixable reason (`... | missing driver: NVENC | fix: <install command>`). It replaces the separate `hw probes:`, capability-limit and per-encoder `hardware probe:` lines; Vulkan warnings are unchanged.
- Report/log polish: probe errors are cut at word boundaries (no `Curren...`), duplicate `[hardware] ...` summary lines are dropped from probe detail when a more specific error exists, and capability-report rows outside the selected device show only the `skipped` note instead of unrelated driver advice (for example Vulkan on an Intel iGPU).
- `VideoPreview` skips a duration ffprobe when `SeekSec` is set and `DurationSec` is unknown.
- Frame and sprite **quality** use a unified **1–100** scale (default 85). MJPEG/WebP mapping is shared across `CaptureFrame`, `Screenshot`, and `SpriteSheet`.
- `CaptureFrameFile` publishes outputs with explicit mode **0644**.
- Sprite sheets **pad** each tile to the configured width/height; hardware decode **filter chains** match `VideoDecoderArgs` per backend (VAAPI/D3D12, Vulkan, VideoToolbox, CUDA).
- HLS VOD bounded jobs stop on the **segment boundary** (no `-t` guard); restarts no longer unlink **ready** cached segments or treat `StopIndex` as a partial encode head under explicit-copy mode.

### Migration

- Prefer `CaptureFrame` / `CaptureFrameFile` with `FrameOptions` instead of `Screenshot` / `VideoPreview`. See [docs/capture.md](docs/capture.md).
