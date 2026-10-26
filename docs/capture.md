# Frame capture and sprite sheets

## Unified API (`CaptureFrame`, `CaptureFrameFile`) — v1.6.0

`CaptureFrame` streams one encoded frame to any `io.Writer`. `CaptureFrameFile` publishes via a temp file in the target directory and atomic rename. Legacy `Screenshot` and `VideoPreview` delegate to the same argv builder but keep their historical defaults (see migration below).

```go
_ = svc.CaptureFrameFile(ctx, "thumb.jpg", ffmpeg.FrameOptions{
    Input:       ffmpeg.InputSource{URL: path, StreamType: ffmpeg.StreamFile},
    At:          ffmpeg.AtPercent(25),
    DurationSec: dur,
})
```

| `At` value | Behavior |
|------------|----------|
| Zero value | First frame (no implicit 10% seek) |
| `AtSec(t)` | Seek to `t` seconds; clamped near EOF when duration is known |
| `AtPercent(p)` | `p`% of duration; values above 100 are clamped to 100 |

ffprobe runs only when the position is percent-based and `DurationSec` is zero.

**Formats:** JPEG (default, `mjpeg` + `image2pipe`), WebP, and PNG when the detected ffmpeg build exposes the encoders.

**Quality:** `Quality` is **1–100** (higher is better). `0` uses the library default (**85**). MJPEG maps to ffmpeg `-q:v` via `round(31 - q×29/100)` clamped to 2–31; WebP uses `-quality q`; PNG ignores quality.

**Optional `MeanLuma`:** set `MeanLuma: &value` on `FrameOptions` to record average luma (0–255) from the encoded frame (buffers once).

## Shared capture argv (`BuildFrameCaptureArgs`)

`Screenshot`, `VideoPreview`, and `CaptureFrame` share optimized argv:

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

- **`CaptureDefaultLightProbeSize`** (`3145728`, 3 MiB)
- **`CaptureDefaultLightAnalyzeDuration`** (`1500000` µs, 1.5 s)

Override per call with `CaptureLightProbe` on `FrameOptions`, `ScreenshotOptions`, `PreviewOptions`, or `SpriteSheetOptions` (`ProbeSize` / `AnalyzeDuration` strings passed straight to ffmpeg).

`ScreenshotOptions` adds optional `SeekSec`, `SeekPercent`, `Width`/`Height`, and `Accuracy` (default `CaptureFast`). `Screenshot` writes directly to the output path; `CaptureFrameFile` uses atomic rename.

### Benchmarks

```bash
go test ./ops -run TestCaptureTimingReport -count=1 -v
go test ./ops -bench=BenchmarkSoftwareFrameCapture -benchtime=3x -count=1
go test -tags=integration ./ops -run TestScreenshotMPEGTSLightProbe -count=1
```

## Migration from legacy APIs

| Legacy | Replacement |
|--------|-------------|
| `Screenshot` | `CaptureFrameFile` with zero `At`, JPEG default |
| `VideoPreview` | `CaptureFrame` with `At: AtSec(...)` or `AtPercent(...)` |

`VideoPreview` still defaults to 10% into the file when neither `SeekSec` nor `SeekPercent` is set. The new API does not — use `AtPercent(10)` explicitly.

`CaptureFrame` is listed in capability detection. `Screenshot` and `VideoPreview` share the same requirements.

## Sprite sheets (`SpriteSheet`)

`Service.SpriteSheet` renders **N** thumbnails in **one** ffmpeg process:

- `select='gte(t,n_forced*INTERVAL)'` for even temporal spacing (or set `IntervalSec` directly)
- `scale` + `pad` to the configured tile size (letterboxed 4:3/16:9 thumbnails keep fixed `#xywh` geometry)
- GPU decode chains match input hwaccel surfaces (`scale_vaapi` / `scale_vulkan` / `scale_vt` / `scale_cuda`, then `hwdownload` where needed)
- `tile=CxR` into a single JPEG or WebP (quality uses the same 1–100 scale as frame capture)
- Optional WebVTT with `#xywh=` cues; when WebVTT is requested, keyframe-only sampling is disabled so cue times match decoded samples

Hardware decode uses the same `VideoDecodeProfile` / detection as transcode. Failures surface as `ErrProfileUnsupported` with stderr hints; **no automatic software fallback**.

`Service.SpriteSheetCapabilities()` exposes preferred and available decode paths so hosts can gate the feature on hardware acceleration.

See `examples/sprite-sheet`.
