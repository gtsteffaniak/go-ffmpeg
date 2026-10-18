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

## Explicit device + ordered fallback (FileBrowser pattern)

Detection is **never scoped**: smoke tests run for every vendor present (Intel, AMD, NVIDIA, Pi V4L2, …). GPU selection only affects which encoder is **chosen** for jobs and what the report marks as `available, not selected`.

```go
devices, _ := ffmpeg.ListDevices() // concrete IDs for admin dropdown

svc, _ := ffmpeg.New(ctx, ffmpeg.Config{
    DeviceSelection: ffmpeg.DeviceSelection{
        Primary:   ffmpeg.GPUByName("NVIDIA GeForce RTX 3050 Laptop GPU"),
        Fallbacks: []ffmpeg.GPUSelector{ffmpeg.GPUSelector{Kind: ffmpeg.GPUIntegrated}},
        AllowSoftware: true, // default; set false to error instead of libx264 fallback
    },
    GPUStrictness: ffmpeg.GPUStrictOnUnavailable,
})
```

Comma-separated lists parse with `ParseGPUSelectorList("dgpu, igpu")`.

Windows pinning uses WMI adapter order for `-gpu` / `d3d11va=dx:<n>` / QSV `child_device=<n>`. DXGI order may differ from CUDA PCI order; the report notes when pinning is best-effort.

## Raspberry Pi (Linux arm)

| Board | H.264 encode | H.264 decode | HEVC decode | Notes |
|-------|--------------|--------------|-------------|-------|
| **Pi 4** | `h264_v4l2m2m` (~1080p30, `-b:v` not CRF) | HW when compiled | HW when compiled | Software fallback above 1080p |
| **Pi 5** | **libx264** (veryfast) | Software | HW when compiled | Report: `H.264: software (no hardware encoder on Raspberry Pi 5)`; recommend ≤720p, low concurrency |
| **Pi 3 and older** | libx264 (ultrafast) | Software | Software | OMX/MMAL removed |

`Capabilities.PiProfile` carries recommended presets, max height, and concurrency hints (callers may override). Use Raspberry Pi OS ffmpeg builds with V4L2 M2M support on Pi 4.
