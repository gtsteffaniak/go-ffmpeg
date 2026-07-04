# test-ffmpeg

HLS transcode validation harness for on-demand fMP4 segments. Generates a **fixture library** from a reference video, runs **benchmarks** (timeline, timing, CPU/GPU), and publishes a **static report site** with graphs, tables, and clickable HLS playback for manual smoothness checks.

## Quick start

From the **go-ffmpeg repo root**:

```bash
make test-hls          # all 21 fixtures + full matrix → test/hls/report_site/data/report.json
make serve-report      # http://127.0.0.1:8765/
```

Optional subset for faster runs:

```bash
make test-hls FIXTURE_NAMES=h264_aac_mp4,hevc_aac_mp4 SEGMENTS=3 FIXTURE_DURATION=10
```

`make report` at the repo root is the **ffmpeg capability report** in the terminal — not this dashboard.

## Commands

| Command | Purpose |
|---------|---------|
| `generate-fixtures` | Reference → 21 named 2-minute samples (h264/hevc/vp9/av1 × aac/mp3/ac3/eac3/opus/vorbis × mp4/mkv/mov/webm/avi) |
| `run` | Fixtures + full benchmark matrix → `report_site/data/report.json` (what `make test-hls` runs) |
| `serve-report` | Static dashboard + HLS player (requires `run` / `test-hls` first) |
| `hls-check` | Single-file on-demand segment timeline validation |
| `continuous-check` | Single-file `ffmpeg -f hls` continuous job (full file) |
| `continuous-check-resume` | Continuous job from `-start-index` / `-start-sec` (EOF resume repro) |
| `matrix` | CLI-only matrix (uses cached HW detect) |
| `playback-test` | Automated hls.js playhead jump test |

### Generate fixtures only

```bash
make test-hls FIXTURE_DURATION=10
# or via the harness binary after make test-hls builds it:
./test/hls/test-ffmpeg generate-fixtures -duration 10 -fixtures test/hls/.fixtures
```

### Full run with report server

```bash
./test-ffmpeg run \
  -reference /path/to/reference.mp4 \
  -fixtures .fixtures \
  -report report_site \
  -duration 120 \
  -segments 0 \
  -serve
```

## What each test measures

For every fixture × mode × accelerator:

- **Conversion errors** — ffmpeg exit / stderr captured
- **Per-segment encode time** (ms) and output size
- **CPU usage** — sampled during encode via OS process stats (`ps` on Linux/macOS, performance counters on Windows); values sum across cores (e.g. 300% ≈ three cores fully busy), matching `ps`/`top` reporting
- **GPU usage** — `nvidia-smi` (NVIDIA), `intel_gpu_top` (Intel Linux), `ioreg` Device Utilization % (macOS Apple GPU). VideoToolbox runs on the media engine; macOS GPU % may under-report during encode even when HW is active.
- **Timeline validation** — fMP4 `tfdt` continuity (`go-ffmpeg/mp4` checks)
- **Browser artifacts** — HLS playlist + segments under `report_site/media/` for manual playback
- **Continuous pipeline** — for remux/copy/transcode/software, also runs `ffmpeg -f hls` with scenarios `full`, `resume_mid`, and `resume_eof` (mirrors filebrowser tail resume). Play via **Play** → `ffmpeg.m3u8`.

### Modes tested per fixture

| Mode | When |
|------|------|
| `remux` | H.264 + AAC in MP4/MOV |
| `copy` | H.264 video + non-AAC audio (audio transcode path) |
| `transcode/software` | Always — output H.264 fMP4 for browser |
| `transcode/qsv`, `vaapi`, … | Each hardware encoder detected **once** at startup |

Hardware detection runs **once** via `sync.Once` and is reused for all matrix/benchmark cases.

## Report site

After `make test-hls`:

- **Dashboard** — pass/fail charts, encode time by fixture, CPU bars
- **Fixture table** — generation status
- **Results table** — click **Play** to open `player.html` with hls.js + playhead jump log
- **Hardware section** — cached capability matrix from startup

```bash
make serve-report   # http://127.0.0.1:8765/
```

## Fixture naming

Each file is `{video}_{audio}_{container}.{ext}`:

```
h264_aac_mp4.mp4      h264_eac3_mkv.mkv     hevc_aac_mp4.mp4
vp9_opus_webm.webm    av1_aac_mkv.mkv       mpeg4_mp3_avi.avi
…21 combinations covering common browser/transcode paths
```

## GPU / hardware verification

The harness reports whether hardware acceleration is actually in use:

| Signal | Meaning |
|--------|---------|
| **Encoder column** | Resolved encoder from `DescribeHLSEncodePlan` (e.g. `h264_qsv`, `h264_vaapi`, `libx264`) |
| **HW active** | `yes` when the encoder is a HW backend, or GPU util &gt; 3% was sampled |
| **GPU avg** | From `nvidia-smi` (NVIDIA), `intel_gpu_top` (Intel), or `ioreg` (macOS). VideoToolbox may show low GPU % while still using HW media blocks — check encoder name and encode time vs software |

Quick HW comparison on one segment:

```bash
./test-ffmpeg hw-check -accel software
./test-ffmpeg hw-check -accel qsv
./test-ffmpeg hw-check -accel vaapi
```

On your Intel Lunar Lake box, QSV and VAAPI should show `encoder=h264_qsv` / `h264_vaapi` in the plan. Install GPU % monitoring:

```bash
sudo apt install intel-gpu-tools
# re-run benchmarks; report GPU column will show e.g. 15% (intel_gpu_top)
```

Without `intel_gpu_top`, **encoder name + faster encode vs software** still confirms the HW path is selected.

## Legacy / focused tests

```bash
make check MODE=remux SEGMENTS=5
make playback-test PLAYBACK_SEGMENTS=8
make debug   # ffmpeg stderr visible
```

## Filebrowser playhead audit

In the filebrowser UI (separate from this harness):

```javascript
localStorage.setItem('hlsTranscodeAudit', '1');
location.reload();
// after playback: window.__hlsTranscodePlayheadAudit.jumps
```

## Dependencies

- Parent module: `github.com/gtsteffaniak/go-ffmpeg` (replaced to `../..` in go.mod)
- `ffmpeg` + `ffprobe` on PATH (or `GOFFMPEG_FFMPEG_PATH` / `GOFFMPEG_FFPROBE_PATH`)
- Optional: `nvidia-smi` for GPU metrics; Node + Playwright for `playback-test`

## CI (software-only)

Every pull request runs the H.264 software matrix via GitHub Actions (`hls-software` job) and locally:

```bash
GOFFMPEG_SKIP_HW=1 HLS_SOFTWARE_ONLY=1 make test-hls SEGMENTS=3 FIXTURE_DURATION=10 \
  FIXTURE_NAMES=h264_aac_mp4,h264_aac_mkv,h264_aac_mov,h264_mp3_mkv,h264_ac3_mkv,h264_eac3_mkv,h264_aac_avi,h264_mp3_avi
```

This uses the bundled sample. CI passes `FIXTURE_NAMES` for a faster H.264 subset; local `make test-hls` runs all 21 fixtures by default.

## Agent workflow

1. Change encode/timeline code in `go-ffmpeg` or `filebrowser/backend/ffmpeg`
2. Root: `make test-hls SEGMENTS=3 FIXTURE_DURATION=10`
3. Open report site — check failures table and manually **Play** borderline cases
4. Ship when remux + transcode/software paths pass for your target fixtures
