# Security

## argv, not shell

All ffmpeg/ffprobe invocations use `exec.CommandContext` with discrete arguments. No shell interpretation.

## Caller responsibilities

| Risk | Mitigation |
|------|------------|
| SSRF via ffmpeg protocols | Validate/limit URLs before `ProbeStream`, transcode, or RTSP inputs |
| Path traversal | Validate `OutputPath`, `OutputDir`, and concat list paths |
| Concat demuxer | `TimelapseCompile` uses `-safe 0`; untrusted concat files can reference arbitrary paths |
| Resource exhaustion | Use `Config.Concurrency` tier limits; Service methods acquire slots automatically |

## Concurrency tiers

`Config.Concurrency` limits parallel subprocesses by workload class:

| Tier | Config field | Default | Operations |
|------|--------------|---------|------------|
| Probe | `MaxProbe` | 16 | `ProbeStream`, `GetMediaDuration`, `DetectSubtitles`, … |
| Decode | `MaxDecode` | 4 | `VideoPreview`, `Screenshot`, `ConvertHEIC`, remux/stream-copy |
| Encode | `MaxEncode` | 2 | `Transcode`, HLS transcode segments, `TimelapseCompile` |
| Global | `GlobalMax` | 0 (off) | Optional cap across all tiers |
| Large file | `MaxLargeFile` | 2 | Extra slot when input exceeds `LargeFileThresholdBytes` |

`Config.MaxConcurrent` is deprecated; when `Concurrency` tier fields are unset it maps to legacy single-pool behavior with a higher probe limit.

`StartHLSContinuous` holds an encode or decode slot until `Wait` completes.

Public `Acquire`/`Release` accept an optional `SlotClass` (default `SlotDecode`) for application-owned ffmpeg outside `Service` methods.
