# Security

## argv, not shell

All ffmpeg/ffprobe invocations use `exec.CommandContext` with discrete arguments. No shell interpretation.

## Caller responsibilities

| Risk | Mitigation |
|------|------------|
| SSRF via ffmpeg protocols | Validate/limit URLs before `ProbeStream`, transcode, or RTSP inputs |
| Path traversal | Validate `OutputPath`, `OutputDir`, and concat list paths |
| Concat demuxer | `TimelapseCompile` uses `-safe 0`; untrusted concat files can reference arbitrary paths |
| Resource exhaustion | Use `MaxConcurrent`; Service methods acquire slots automatically |

## Semaphore

`Config.MaxConcurrent` limits parallel ffmpeg processes. `StartHLSContinuous` holds a slot until the job finishes alignment.

Public `Acquire`/`Release` are for application-owned ffmpeg outside `Service` methods.
