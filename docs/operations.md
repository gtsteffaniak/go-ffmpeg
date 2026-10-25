# Operations

go-ffmpeg exposes **tasks**, not ffmpeg primitives. Each method accepts a typed options struct; the library chooses encoders, hardware paths, and the optimized flag set for that operation. When the host lacks a codec or HW backend, capability detection reports why and operations fall back or return `ErrUnsupported` before spawning ffmpeg.

High-level tasks on `Service`:

| Task | Service method | Notes |
|------|----------------|-------|
| Probe stream | `ProbeStream`, `ProbeFile` | RTSP, HLS, file, HTTP |
| Duration / dimensions | `GetMediaDuration`, `GetImageDimensions` | ffprobe |
| Frame capture | `CaptureFrame`, `CaptureFrameFile` | JPEG/WebP/PNG to `io.Writer` or file (atomic); `AtSec` / `AtPercent`, fast or exact seek |
| Screenshot | `Screenshot` | **Deprecated** — use `CaptureFrameFile`; optimized image2pipe capture |
| Preview | `VideoPreview` | **Deprecated** — use `CaptureFrame` (MJPEG); optional scale and seek |
| Sprite sheet | `SpriteSheet` | Tiled JPEG/WebP + optional WebVTT; one ffmpeg pass; HW decode when configured |
| Convert | `ConvertHEIC` | HEIC/HEIF → JPEG; more formats planned |
| Transcode | `Transcode` | `VideoProfile` + optional decode profile |
| Segmented record | `SegmentRecord` | MP4 segments |
| Live fMP4 | `FMP4StreamCopy`, `FMP4Transcode` | Stream to `io.Writer` |
| HLS on-demand | `HLSSegment`, `HLSInitAndSegment`, `HLSSegmentMedia` | fMP4 fragments for MSE |
| HLS continuous | `StartHLSContinuous` | Long-running `-f hls` disk cache |
| Timelapse | `TimelapseCompile` | Concat list → video |
| Subtitles | `DetectSubtitles`, `ExtractSubtitle` | WebVTT extract |

Unsupported operations return `ErrUnsupported` with reasons from capability detection. Unsupported profiles return `ErrProfileUnsupported` before ffmpeg runs.

More high-level jobs (additional convert paths, audio extract, etc.) follow the same pattern: options struct → `ops` → register → `Service` method.
