# Hardware acceleration

go-ffmpeg detects which encoders and decode/encode backends your ffmpeg build exposes, optionally runs HW smoke tests, and picks the best path per task.

## Default backend order (no GPU scoping)

| OS | Default order |
|----|----------------|
| **Linux** | VAAPI → QSV → NVENC → AMF |
| **Windows** | QSV → NVENC → AMF |
| **WSL2** | VAAPI → QSV → D3D12 → NVENC |
| **macOS** | VideoToolbox |

Selection is **per codec**, backend-first: `BuildCodecMatrix` walks the hierarchy and picks the first available encoder/decoder for each codec.

## GPU configuration (typed API)

```go
import "github.com/gtsteffaniak/go-ffmpeg"

// Autoselect (zero value) — platform default order on all GPUs.
svc, _ := ffmpeg.New(ctx, ffmpeg.Config{
    GPUSelector: ffmpeg.GPUSelector{Kind: ffmpeg.GPUAuto},
})

// Integrated / discrete / software
svc, _ = ffmpeg.New(ctx, ffmpeg.Config{
    GPUSelector: ffmpeg.GPUSelector{Kind: ffmpeg.GPUIntegrated},
    GPUStrictness: ffmpeg.GPUStrictOnUnavailable, // error if no iGPU; default is fallback to auto
})

svc, _ = ffmpeg.New(ctx, ffmpeg.Config{
    GPUSelector: ffmpeg.GPUByRenderNode("/dev/dri/renderD129"),
})

svc, _ = ffmpeg.New(ctx, ffmpeg.Config{
    GPUSelector: ffmpeg.GPUByName("RTX 3050"),
})

svc, _ = ffmpeg.New(ctx, ffmpeg.Config{
    GPUSelector: ffmpeg.GPUSelector{Kind: ffmpeg.GPUSoftware},
})
```

### FileBrowser YAML (string wrapper)

FileBrowser can keep a YAML string and map it with the optional parser:

```yaml
gpu: ""   # default=autoselect
# gpu: igpu
# gpu: dgpu
# gpu: software
# gpu: /dev/dri/renderD129
# gpu: "RTX 3050"
```

```go
sel, err := ffmpeg.ParseGPUSelector(yamlGPU)
if err != nil { /* handle */ }
choice, err := ffmpeg.ResolveGPUSelector(sel, ffmpeg.GPUFallbackOnUnavailable)

svc, _ := ffmpeg.New(ctx, ffmpeg.Config{
    GPUSelector: sel,
    GPUStrictness: ffmpeg.GPUFallbackOnUnavailable,
})
```

Deprecated: `Config.GPU` string is still accepted when `GPUSelector` is zero.

## Override precedence

1. **`EncoderHierarchy`**
2. **`GPUSelector`** (scopes vendor/device; `GPUSoftware` skips HW tests)
3. **`EncoderPreference`** (`performance` = NVENC-first on Linux)
4. **Platform default**

## Detection

`platform.Detect()` → FFmpeg inventory → optional smoke tests → codec matrix. The capability report **GPU configuration** section shows the resolved selector, mode, fallbacks, and device.

Skip HW tests: `SkipHWTests`, `GPUSoftware`, or `GOFFMPEG_SKIP_HW=1`.
