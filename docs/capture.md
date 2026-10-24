# Frame capture and sprite sheets

## Frame capture (`Screenshot`, `VideoPreview`)

Both paths share optimized argv built by `ops.BuildFrameCaptureArgs`:

| Technique | Purpose |
|-----------|---------|
| Input `-ss` before `-i` | Fast demuxer seek |
| `-noaccurate_seek` (default) | Nearest keyframe thumbnail (`CaptureFast`) |
| `-accurate_seek` | Decode to exact timestamp (`CaptureExact`) |
| `-skip_frame nokey` | Keyframe-only decode in fast mode |
| `-probesize 32768` / `-analyzeduration 0` | Lightweight input when duration/seek is known |
| `-sws_flags fast_bilinear` | Cheaper scale during decode |
| `-an -sn -dn`, `-threads 1` | Minimal work for one frame |
| `image2pipe` to stdout | Avoid ffmpeg temp files; Go streams to `io.Writer` or output path |

`ScreenshotOptions` adds optional `SeekSec`, `SeekPercent`, `Width`/`Height`, and `Accuracy` (default `CaptureFast`).

### Benchmarks

Run on the bundled sample (requires `ffmpeg` on `PATH`):

```bash
go test ./ops -run TestCaptureTimingReport -count=1 -v
go test ./ops -bench=BenchmarkSoftwareFrameCapture -benchtime=3x -count=1
```

`TestCaptureTimingReport` logs probe vs ffmpeg time for software capture at several seek points. Measured on CI (ffmpeg 6.1, `Big_Buck_Bunny_1080_10s_2MB.mp4`, duration known, `-probesize 32K`):

| Scenario | ffprobe | ffmpeg (seek+decode) | total |
|----------|---------|----------------------|-------|
| seek 7s, `CaptureFast` | 58ms | 124ms | 182ms |
| seek 7s, `CaptureExact` | 56ms | 341ms | 398ms |
| seek 0.5s, `CaptureFast` | 59ms | 161ms | 220ms |

`BenchmarkSoftwareFrameCapture` (5 iterations, includes probe each op): **252ms/op** fast vs **808ms/op** exact at seek 7s (~3.2× faster).

Exact mode costs more when the GOP between the keyframe and target is long; fast mode stops at the prior keyframe.

## Sprite sheets (`SpriteSheet`)

`Service.SpriteSheet` renders **N** thumbnails in **one** ffmpeg process:

- `select='gte(t,n_forced*INTERVAL)'` for even temporal spacing (or set `IntervalSec` directly)
- `scale` (or GPU `scale_vaapi` / `scale_vt` / `scale_cuda` when decoding on hardware)
- `tile=CxR` into a single JPEG or WebP
- Optional WebVTT with `#xywh=` cues for scrub previews

Hardware decode uses the same `VideoDecodeProfile` / detection as transcode. Failures surface as `ErrProfileUnsupported` with stderr hints; **no automatic software fallback**.

`Service.SpriteSheetCapabilities()` exposes preferred and available decode paths so hosts can gate the feature on hardware acceleration.

See `examples/sprite-sheet`.
