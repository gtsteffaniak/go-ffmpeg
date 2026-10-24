# Frame capture and sprite sheets

## Frame capture (`Screenshot`, `VideoPreview`)

Both paths share optimized argv built by `ops.BuildFrameCaptureArgs`:

| Technique | Purpose |
|-----------|---------|
| Input `-ss` before `-i` | Fast demuxer seek |
| `-noaccurate_seek` (default) | Nearest keyframe thumbnail (`CaptureFast`) |
| `-accurate_seek` | Decode to exact timestamp (`CaptureExact`) |
| `-skip_frame nokey` | Keyframe-only decode in fast mode |
| Lightweight `-probesize` / `-analyzeduration` | Skip full demuxer probe when duration/seek is known |
| `-sws_flags fast_bilinear` | Cheaper scale during decode |
| `-an -sn -dn`, `-threads 1` | Minimal work for one frame |
| `image2pipe` to stdout | Avoid ffmpeg temp files; Go streams to `io.Writer` or output path |

### Lightweight probing

When the caller already knows duration or a seek target, capture sets smaller-than-default probe limits:

- **`CaptureDefaultLightProbeSize`** (`3145728`, 3 MiB) — enough for MPEG-TS programs, MKV with late streams, and fragile AVI indexes. Values around 32 KiB often fail stream detection or yield gray/garbage frames on those containers.
- **`CaptureDefaultLightAnalyzeDuration`** (`1500000` µs, 1.5 s) — bounded analyze time without disabling probe entirely (`0` is unsafe on TS).

Override per call with `CaptureLightProbe` on `ScreenshotOptions`, `PreviewOptions`, or `SpriteSheetOptions` (`ProbeSize` / `AnalyzeDuration` strings passed straight to ffmpeg).

`ScreenshotOptions` adds optional `SeekSec`, `SeekPercent`, `Width`/`Height`, and `Accuracy` (default `CaptureFast`).

### Benchmarks

Run on the bundled sample (requires `ffmpeg` on `PATH`):

```bash
go test ./ops -run TestCaptureTimingReport -count=1 -v
go test ./ops -bench=BenchmarkSoftwareFrameCapture -benchtime=3x -count=1
go test -tags=integration ./ops -run TestScreenshotMPEGTSLightProbe -count=1
```

`TestCaptureTimingReport` logs probe vs ffmpeg time for software capture at several seek points. Measured on CI (ffmpeg 6.1, `Big_Buck_Bunny_1080_10s_2MB.mp4`, duration known, 3 MiB / 1.5 s light probe):

| Scenario | ffprobe | ffmpeg (seek+decode) | total |
|----------|---------|----------------------|-------|
| seek 7s, `CaptureFast` | 57ms | 127ms | 184ms |
| seek 7s, `CaptureExact` | 57ms | 347ms | 405ms |
| seek 0.5s, `CaptureFast` | 71ms | 182ms | 253ms |

`BenchmarkSoftwareFrameCapture` (5 iterations, includes probe each op): **~247ms/op** fast vs **~577ms/op** exact at seek 7s (~2.3×).

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
