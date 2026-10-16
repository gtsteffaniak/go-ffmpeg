# Hardware acceleration

go-ffmpeg detects which encoders and decode/encode backends your ffmpeg build exposes, optionally runs HW smoke tests, and picks the best path per task. When hardware is missing or unsuitable, encode falls back to software without changing your call site.

## Default backend order (no config)

Chosen by `HierarchyForPlatform` — same on hybrid and single-GPU; only **availability** differs per backend after detection.

| OS | Default order |
|----|----------------|
| **Linux** | VAAPI → QSV → NVENC → AMF |
| **Windows** | QSV → NVENC → AMF |
| **WSL2** | VAAPI → QSV → D3D12 → NVENC |
| **macOS** | VideoToolbox |

Hybrid laptops use the same backend list as single-GPU hosts. Selection is **per codec**, not per device: `BuildCodecMatrix` walks the hierarchy and picks the first available backend for each codec (`resolvePreferredEncode` / `resolvePreferredDecode`). Intel iGPU paths (VAAPI/QSV) are ahead of NVENC on Linux by default, so H.264/HEVC prefer the iGPU when those encoders pass smoke tests—even though NVENC is still detected on the dGPU. Only `Config.GPU` scopes detection to one device/vendor. On hybrid Linux, the default DRM render node for VAAPI/QSV smoke tests is the **integrated** GPU when both are present (`DefaultRenderNodeForGPUs`).

## Override precedence (highest first)

1. **`EncoderHierarchy`** — explicit `[]AccelType` order (or `{AccelNone}` for software-preferred while still detecting HW).
2. **`GPU`** — scopes to one device (`igpu`, `dgpu`, render node, name substring); sets vendor backends via `HierarchyForGPU` (e.g. NVIDIA → `[NVENC]`, Intel → `[VAAPI, QSV]`).
3. **`EncoderPreference`** — only when (1) and (2) are unset: `performance` / `dgpu-first` → NVENC-before-VAAPI on Linux; `powersave` / `igpu-first` → same as platform default.
4. **Platform default** — table above.

Per-request overrides: `VideoProfile.Accel`, `ForceSoftware`.

## Configuration

```go
svc, _ := ffmpeg.New(ctx, ffmpeg.Config{
    GPU:               "igpu",        // optional device scope
    EncoderPreference: "performance", // optional; ignored if GPU or EncoderHierarchy set
    // EncoderHierarchy: []ffmpeg.AccelType{ffmpeg.AccelNVENC, ffmpeg.AccelVAAPI},
})
```

Empty `GPU` means **no device scoping** — detection probes the full platform; the hierarchy above picks preferred codecs.

## Detection (all OS)

1. **`platform.Detect()`** — OS/arch, vendor flags (NVIDIA/Intel/AMD), VAAPI driver, QSV runtime, DRI nodes, PCI GPU list (display class `0x03xx` only), distro for install hints.
2. **FFmpeg inventory** — `-encoders` / `-decoders` / filters / protocols from the binary.
3. **Optional smoke tests** — short lavfi encode per HW encoder; VAAPI uses `platform.RenderDevice` (first render node, or selected GPU’s node). Platform gates skip impossible backends (e.g. QSV without `libmfx-gen`, AMF without AMD).
4. **GPU scoping** — when `GPU` is set, `ScopedPlatform` narrows vendor flags; encoders/decoders outside the scoped hierarchy are listed but marked skipped (not smoke-tested).
5. **Codec matrix** — walks the resolved hierarchy; first **available** encoder/decoder per codec becomes `Preferred` / `DecodePreferred`.

## Report

The capability report lists smoke-test results per backend and, under **Codec resolution**, each codec’s chosen path with backend, GPU name, and DRM render node when known.

Skip slow tests: `SkipHWTests` or `GOFFMPEG_SKIP_HW=1`. CLI: `go-ffmpeg -skip-hw-tests`.

## Backends

| Backend | Typical platform |
|---------|------------------|
| NVENC | NVIDIA |
| AMF | AMD (Windows; limited Linux) |
| QSV | Intel |
| VAAPI | Intel/AMD Linux |
| VideoToolbox | macOS |
| D3D12 | WSL2 |

## Non-goals (today)

Vulkan encode, RKMPP, V4L2 mem2mem, separate NVDEC vs NVENC policy APIs.
