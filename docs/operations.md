# Operations

High-level tasks exposed on `Service`. All take typed options; the library builds ffmpeg argv.

| Task | Service method | Notes |
|------|----------------|-------|
| Probe stream | `ProbeStream`, `ProbeFile` | RTSP, HLS, file, HTTP |
| Duration / dimensions | `GetMediaDuration`, `GetImageDimensions` | ffprobe |
| Screenshot | `Screenshot` | Single JPEG/PNG frame |
| Preview | `VideoPreview` | MJPEG to `io.Writer` |
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
