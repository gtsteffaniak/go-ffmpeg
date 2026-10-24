# Hardware encoder detection

go-ffmpeg uses **layered detection** so `-hwaccels` / `-encoders` (compile-time lists) are not mistaken for working GPU encode paths. Detection runs **once** when the service is initialized (`ffmpeg.New` / `capabilities.Detect`). The resulting capability matrix is **final for that process**: go-ffmpeg does **not** re-probe encoders, mutate availability after transcode failures, or automatically re-run jobs with software after a hardware error.

To pick up driver or container changes, create a new service (`New`) or call `Service.Reload` explicitly — there is no per-encoder or automatic retry API.

## Layers

### 1. Binary capabilities

- Parse `ffmpeg -encoders` / `-hwaccels` (compile-time inventory).
- On **Linux**, detect **static vs dynamic** linking:
  - ELF `PT_INTERP` program header (no interpreter ⇒ static).
  - `ldd` fallback (`not a dynamic executable`).
- **Static Linux ffmpeg** cannot `dlopen` GPU userspace drivers (`*_drv_video.so`, `libnvidia-encode`, `libmfx-gen`, Vulkan ICDs). VAAPI/QSV/NVENC/AMF probes stop with code `static_binary` (except **V4L2 M2M**, which works without those dlopens).
- Windows/macOS use normal dynamic linking; this gate is skipped.

### 2. Host prerequisites

Per backend before any encode probe:

| Backend | Linux checks |
|---------|----------------|
| **VAAPI / QSV** | `/dev/dri/renderD*` exists and is openable; `libva.so.2`; VA driver blobs (`LIBVA_*` paths) |
| **NVENC** | `libcuda.so.1` and `libnvidia-encode.so.1` resolvable (`ldconfig -p` / standard paths) |
| **V4L2 M2M** | `/dev/video*` openable |
| **WSL** | `/dev/dxg`, `/usr/lib/wsl/lib`, Mesa `d3d12_drv_video.so` when using the D3D12 VA path |

Permission failures on render nodes include a hint to add the service user to the **render** group and pass `--device /dev/dri/...` in containers.

### 3. Functional probe

For each compiled hardware encoder candidate, ffmpeg runs a tiny real encode:

`testsrc` ~0.5s, small resolution → encoder → `-f null`

Runs in a **process group** with a hard timeout (kills the group on expiry). Only encoders that pass are marked **available**.

## Structured results

Each hardware encoder gets an `EncoderProbeResult`:

| Field | Meaning |
|-------|---------|
| `encoder` | ffmpeg encoder name |
| `accel` | Backend (`vaapi`, `nvenc`, …) |
| `available` | Passed functional probe |
| `code` | Failure class (empty when available) |
| `hint` | One-line remediation |
| `detail` | Truncated stderr snippet |

Access via `capabilities.Capabilities.EncoderProbeResults()` or JSON on the capability report.

**UI log line:** `Capabilities.HardwareProbeSummaryLine()` — e.g. `hw probes: ffmpeg=dynamic | available: h264_nvenc, hevc_nvenc`.

When a GPU is present in sysfs but probes fail, startup logs a **warn** line per encoder (`Service.Reload` / `New`) and probe `hint` fields include distro-specific remediation. Code may be upgraded to `gpu_present_driver_missing`.

### GPU presence (driver-independent)

Detected from `/sys/class/drm/card*/device/vendor` (`0x8086` Intel, `0x1002` AMD, `0x10de` NVIDIA), `/dev/nvidia*`, and `/dev/dxg` (WSL). This is separate from `platform.Detect()` driver gates.

### Host context

- **Distro:** `/etc/os-release` `ID` / `ID_LIKE` (Fedora/RHEL, Arch, Debian/Ubuntu, Alpine, …)
- **Container:** `/.dockerenv` or `/proc/1/cgroup` (docker, containerd, kubernetes)

### Remediation matrix (informational)

| Vendor | Fedora / RHEL | Arch | Debian / Ubuntu | Containers |
|--------|----------------|------|-----------------|------------|
| **Intel** H.264/HEVC/QSV | `intel-media-driver` + `intel-vpl-gpu-rt` | `intel-media-driver`, `vpl-gpu-rt` | `intel-media-va-driver-non-free`, `libmfx-gen1.2` | `--device /dev/dri` + render group |
| **AMD** VAAPI encode | `mesa-va-drivers-freeworld` (**RPM Fusion**; stock Mesa often lacks encode) | `libva-mesa-driver` | `mesa-va-drivers` | `--device /dev/dri` + render group |
| **NVIDIA** NVENC | Proprietary driver + `libnvidia-encode` | `nvidia` driver package | `nvidia-driver` + encode libs | `--gpus all`, NVIDIA Container Toolkit, **non-static** ffmpeg image |
| **Any** + static ffmpeg | — | — | — | Use full dynamic/glibc ffmpeg build |

Alpine and other families get generic `apk`/`install` hints derived from the same tables in `platform.DistroContext`.

## Error codes

| Code | Typical cause | Fix |
|------|----------------|-----|
| `static_binary` | Static Linux ffmpeg | Use a glibc/full **dynamic** distro build |
| `no_device` | No render node / GPU device | Load driver, pass `--device /dev/dri/renderD128`, WSL: enable GPU-PV |
| `device_permission_denied` | Cannot open render node | Add user to `render`/`video` group |
| `libva_missing` | libva not installed | Install `libva2` + drivers |
| `va_driver_missing` | No `*_drv_video.so` | Install Intel/AMD/Mesa VA driver; set `LIBVA_DRIVER_NAME` |
| `va_driver_init_failed` | VA init failed (often **-12**) | Driver/GPU mismatch, permissions, or static ffmpeg |
| `nvidia_libs_missing` | CUDA/NVENC libs not found | Install driver; containers: `--gpus all` + NVIDIA Container Toolkit |
| `nvenc_unsupported_gpu` | GPU/driver cannot encode | Update driver or pick another encoder |
| `nvenc_session_limit` | Too many NVENC sessions | Reduce concurrent NVENC jobs |
| `timeout` | Probe hung | Driver/GPU health, load |
| `gpu_present_driver_missing` | GPU in sysfs but VA/NVENC stack missing | See remediation matrix above; check container devices and static vs dynamic ffmpeg |
| `unknown` | Unclassified stderr | Inspect `detail` |

These codes describe **startup detection**. A later transcode may still fail at runtime; the library returns that error to the caller and does not change the cached capability matrix or retry the job.

## Platform matrix

| Environment | Notes |
|-------------|--------|
| **Linux native** | Full layered detection; prefers `/dev/dri/renderD*` |
| **Docker on Linux** | Pass GPU devices + driver libs; static ffmpeg images often show `static_binary` |
| **WSL2** | `/dev/dxg` + `/usr/lib/wsl/lib`; VAAPI via Mesa **d3d12** when no `/dev/dri` |
| **Docker on WSL** | Combine WSL GPU rules with container device mounts |
| **Windows** | QSV/NVENC/AMF via normal DLL load; prerequisite layer skipped |
| **macOS** | VideoToolbox; prerequisite layer skipped |
| **Raspberry Pi** | V4L2 M2M allowed on static builds; Pi profile docs still apply |

## Transcode failures

When a hardware encode fails, `OperationError` / stderr may still be classified (e.g. `FailureHardware`) for logging or HTTP mapping. go-ffmpeg **does not**:

- mark encoders unhealthy or flip `available` flags
- re-run detection or smoke tests
- automatically retry the same transcode with software

Callers that want software after a failure must start a **new** operation with an explicit software profile or restart the service after fixing the environment (`Reload`).

## Detection vs selection

- `platform.Detect()` and smoke tests stay **unscoped** (all vendors probed).
- `DeviceSelection` / `GPUSelector` only affect which **detected-available** encoder is **preferred**, not whether others are probed.
- Per-job **software fallback** during resolution (e.g. `AllowSoftware`) uses the one-shot detection snapshot only — it is not a runtime re-probe or retry after failure.

See also [hardware.md](hardware.md) for GPU configuration and FileBrowser-style device lists.
