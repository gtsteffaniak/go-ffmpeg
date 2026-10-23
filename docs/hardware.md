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

### Zero config (`GPUAuto`)

`ffmpeg.New(ctx, ffmpeg.Config{})` is the recommended default. Zero `Config`, zero `DeviceSelection`, and zero `GPUSelector` all mean **`GPUAuto`**: full unscoped detection, device order **iGPU/SoC → dGPU → software**, vendor backend order on each device, and automatic per-job fallback along the chain.

```go
import "github.com/gtsteffaniak/go-ffmpeg"

// GPUAuto — same as omitting GPUSelector entirely
svc, _ := ffmpeg.New(ctx, ffmpeg.Config{})

sel, _ := ffmpeg.ParseGPUSelector("auto") // or "" → GPUAuto
svc, _ = ffmpeg.New(ctx, ffmpeg.Config{GPUSelector: sel})

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

## Detection

`platform.Detect()` → FFmpeg inventory → optional smoke tests → codec matrix. The capability report **GPU configuration** section shows the resolved selector, mode, fallbacks, and device.

Skip HW tests: `SkipHWTests` or `GOFFMPEG_SKIP_HW=1`. A software selection still detects hardware so hosts can show what is available.

## Default device order (`GPUAuto`)

With `GPUAuto` (zero `GPUSelector` and zero `DeviceSelection`), detection builds a **device-first** chain:

1. Integrated GPU(s) and SoC blocks (Raspberry Pi V4L2 M2M counts as integrated)
2. Discrete GPU(s)
3. Software (`libx264`, …)

Within each device, backends follow vendor order (e.g. Intel Linux: VAAPI → QSV; AMD Windows: AMF; NVIDIA: NVENC). The capability report includes a line such as:

`Device order (auto): AMD Radeon Graphics → NVIDIA GeForce RTX 3050 Laptop GPU → software`

Opt into discrete-first order:

```go
svc, _ := ffmpeg.New(ctx, ffmpeg.Config{
    DevicePreference: ffmpeg.PreferDiscrete,
})
```

`EncoderPreference: "performance"` is a **deprecated alias** for `PreferDiscrete` (device order only). Override precedence:

1. **`DeviceSelection`** (primary + fallbacks) or legacy non-auto **`GPUSelector`**
2. **`EncoderHierarchy`**
3. **`DevicePreference`** / deprecated `EncoderPreference` performance
4. Platform defaults

## Explicit device + ordered fallback (FileBrowser pattern)

Detection is **never scoped**: smoke tests run for every vendor present (Intel, AMD, NVIDIA, Pi V4L2, …). GPU selection only affects which encoder is **chosen** for jobs and what the report marks as `available, not selected`.

```go
devices, _ := ffmpeg.ListDevices() // concrete IDs for admin dropdown

svc, _ := ffmpeg.New(ctx, ffmpeg.Config{
    DeviceSelection: ffmpeg.DeviceSelection{
        Primary:   ffmpeg.GPUByName("NVIDIA GeForce RTX 3050 Laptop GPU"),
        Fallbacks: []ffmpeg.GPUSelector{ffmpeg.GPUSelector{Kind: ffmpeg.GPUIntegrated}},
        // AllowSoftware: &false, // optional override; else use Config.AllowSoftwareEncoderFallback
    },
    GPUStrictness: ffmpeg.GPUStrictOnUnavailable,
})
```

Comma-separated lists parse with `ParseGPUSelectorList("dgpu, igpu")`.

Windows pinning prefers **DXGI adapter enumeration** (bundled script) for `d3d11va=dx:<n>` / QSV `child_device=<n>` and falls back to WMI adapter order when DXGI is unavailable. NVENC uses `nvidia-smi` PCI matching for `-gpu <cuda_index>` when exact. The capability report notes best-effort pinning.

## Raspberry Pi (Linux arm)

| Board | H.264 encode | H.264 decode | HEVC decode | Notes |
|-------|--------------|--------------|-------------|-------|
| **Pi 4** | `h264_v4l2m2m` (~1080p30, `-b:v` not CRF) | HW when compiled | HW when compiled | Software fallback above 1080p |
| **Pi 5** | **libx264** (veryfast) | Software | HW when compiled | Report: `H.264: software (no hardware encoder on Raspberry Pi 5)`; recommend ≤720p, low concurrency |
| **Pi 3 and older** | libx264 (ultrafast) | Software | Software | OMX/MMAL removed |

`Capabilities.PiProfile` carries recommended presets, max height, and concurrency hints (callers may override). Use Raspberry Pi OS ffmpeg builds with V4L2 M2M support on Pi 4.
