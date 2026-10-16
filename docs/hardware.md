# Hardware acceleration

go-ffmpeg detects which encoders and decode/encode backends your ffmpeg build actually exposes, optionally runs HW smoke tests, and picks the best path per task. When hardware is missing, disabled (`GPU` empty), or unsuitable for a profile, encode falls back to software without changing your call site.

## Backends

| Backend | Typical platform |
|---------|------------------|
| NVENC | NVIDIA |
| AMF | AMD (Windows; limited Linux) |
| QSV | Intel |
| VAAPI | Intel/AMD Linux |
| VideoToolbox | macOS |
| D3D12 | WSL2 |

## Configuration

```go
svc, _ := ffmpeg.New(ctx, ffmpeg.Config{
    // Optional: scope smoke tests and preferred paths to one GPU.
    GPU: "igpu", // or "dgpu", "/dev/dri/renderD129", "GeForce RTX 4090"

    // Optional: on hybrid laptops, prefer Intel VAAPI/QSV over NVENC (battery/heat).
    EncoderPreference: "powersave", // aliases: igpu-first, integrated

    // Optional: explicit backend order (overrides EncoderPreference).
    // EncoderHierarchy: []ffmpeg.AccelType{ffmpeg.AccelVAAPI, ffmpeg.AccelNVENC},
})
```

When `GPU` is empty, detection runs against the full platform (all GPUs); the codec matrix picks backends from `EncoderPreference` or the default hierarchy (NVENC before VAAPI on Linux hybrid).

Override per request with `VideoProfile.Accel` or `ForceSoftware`.

The capability report **Codec resolution** section lists each codec’s preferred encode/decode path with backend, GPU name, and DRM render node when known.

## Detection

Startup detection lists compiled encoders, runs optional HW smoke tests, and builds a codec matrix. Skip slow tests with `SkipHWTests` or `GOFFMPEG_SKIP_HW=1`.

CLI: `go-ffmpeg -skip-hw-tests`

## Non-goals (today)

Vulkan encode, RKMPP, V4L2 mem2mem, separate NVDEC vs NVENC policy APIs.
