# Frame capture

v1.6.0 unifies single-frame extraction behind `CaptureFrame` and `CaptureFrameFile`. Legacy `Screenshot` and `VideoPreview` remain as deprecated wrappers with the same behavior.

## API

```go
// Stream JPEG to an HTTP response
_ = svc.CaptureFrame(ctx, w, ffmpeg.FrameOptions{
    Input: ffmpeg.InputSource{URL: path, StreamType: ffmpeg.StreamFile},
    At:    ffmpeg.AtPercent(25),
    DurationSec: dur, // skip ffprobe when set
})

// Atomic file write
_ = svc.CaptureFrameFile(ctx, "thumb.jpg", ffmpeg.FrameOptions{
    Input: ffmpeg.InputSource{URL: path, StreamType: ffmpeg.StreamFile},
    At:    ffmpeg.AtSec(4.5),
    Quality: 85,
})
```

### Position

| `At` value | Behavior |
|------------|----------|
| Zero value | First frame (no implicit 10% seek) |
| `AtSec(t)` | Seek to `t` seconds; clamped near EOF when duration is known |
| `AtPercent(p)` | `p`% of duration; values above 100 are clamped to 100 |

`DurationSec` on `FrameOptions` avoids an ffprobe round-trip when you already know the length. ffprobe runs only when the position is percent-based and `DurationSec` is zero.

### Accuracy

- `CaptureFast` (default): `-ss` before `-i` (fast input seek).
- `CaptureExact`: `-ss` after `-i` (frame-accurate, slower).

### Format

Default output is JPEG (`mjpeg` in `image2`). WebP and PNG are selected with `Format: ffmpeg.FrameWebP` or `ffmpeg.FramePNG` when the detected ffmpeg build exposes `libwebp`/`webp` or `png` encoders.

### Light probe

Set `LightProbe: &ffmpeg.CaptureLightProbe{}` to populate `MeanLuma` (0–255) from the encoded frame. The library buffers the frame when a probe is requested.

## Migration from legacy APIs

| Legacy | Replacement |
|--------|-------------|
| `Screenshot` | `CaptureFrameFile` with zero `At`, JPEG default |
| `VideoPreview` | `CaptureFrame` with `At: AtSec(...)` or `AtPercent(...)`; use `DurationSec` to skip probe |

`VideoPreview` still defaults to 10% into the file when neither `SeekSec` nor `SeekPercent` is set. The new API does not — use `AtPercent(10)` explicitly.

`Screenshot` writes directly to the output path (non-atomic). `CaptureFrameFile` always publishes via rename.

## Capability

`CaptureFrame` is listed in capability detection. `Screenshot` and `VideoPreview` share the same requirements and stay enabled when capture is supported.
