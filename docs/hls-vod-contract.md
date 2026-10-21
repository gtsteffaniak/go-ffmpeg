# HLS on-demand VOD contract (Model A)

This document defines the **playlist-local time** model for browser MSE playback of files on disk (Jellyfin-style on-demand VOD). FileBrowser Quantum and other hosts should treat this as the target integration contract; legacy EVENT/continuous and per-segment buffer APIs remain supported but are not recommended for new on-demand file playback.

## Goals

- hls.js maps fMP4 fragments by **playlist position**, not by absolute decode timestamps from a growing EVENT playlist.
- The host serves a **synthetic full-length VOD media playlist** (`#EXT-X-PLAYLIST-TYPE:VOD`, `#EXT-X-ENDLIST`) built from probe data, independent of encode progress.
- ffmpeg jobs write numbered `seg/%05d.m4s` fragments with `-output_ts_offset` and `-start_number` so muxed output lines up with the playlist index map. **hls.js places fragments by playlist position**; each segment file's fMP4 decode timeline is segment-local (near zero), not an absolute file offset.
- No per-segment `tfdt` rebasing on transcode paths (legacy continuous aligner is not used).

## Timeline rules

| Mode | Segment boundaries | Playlist `EXTINF` |
|------|-------------------|-------------------|
| Full transcode | Fixed grid: segment `k` starts at `k * segmentSec` | `segmentSec` except last (remainder) |
| Remux / video-copy | Uses `segmentSec` as a cumulative target, then snaps each cut to the next video keyframe | Actual span between emitted keyframe boundaries (can exceed `segmentSec`) |

All times are on a **0-based media timeline** (container `start_time` subtracted before planning).

Segment index `k` playlist start time:

- **Transcode:** `k * segmentSec`
- **Copy:** `starts[k]` from `BuildVODSegmentList` / keyframe planner

## On-disk layout

```
<cache-job>/
  init.m4s              # fMP4 init (EXT-X-MAP)
  seg/00000.m4s         # media fragments
  seg/00001.m4s
  ...
  .ffmpeg-vod.m3u8      # internal ffmpeg playlist (do not serve)
```

Segment URI pattern in the host playlist: `seg/%05d.m4s` (see `HLSVODSegmentNamePattern`).

## Frozen ffmpeg argv (library-owned)

Hosts must not hand-roll mux flags; use `BuildHLSVODFFmpegArgs` / `StartHLSVODSegmentJob`.

### Common (all pipelines)

1. `-ss <inputSeek>` **before** `-i` (accurate seek for transcode; keyframe seek for copy).
2. `-output_ts_offset <StartSec>` — playlist-local anchor for segment `StartIndex` (one constant per job).
3. `-start_number <StartIndex>` — filenames align with playlist indices.
4. `-f hls -hls_segment_type fmp4 -hls_fmp4_init_filename init.m4s -hls_segment_filename seg/%05d.m4s`
5. `-hls_flags temp_file+independent_segments` — atomic segment publish (temp file then rename).
6. Internal playlist path: `.ffmpeg-vod.m3u8` (not served).

### Full transcode

- Forced keyframes: `-force_key_frames expr:gte(t,n_forced*N)` with `N = segmentSec` (relative to post-seek timeline).
- `-g` / `-keyint_min` from GOP; `-video_track_timescale 90000` for fMP4.
- AAC: `-c:a aac -ar 48000 -ac 2 -profile:a aac_low -af aresample=async=1` (add `:first_pts=0` only when `StartSec == 0`).
- **Do not** use `-copyts` or post-encode `tfdt` rebasing for transcode VOD jobs.
- **Do not** use `-max_interleave_delta 0` or huge `aresample=async=192000` windows.

### Remux / video-copy

- `-fflags +genpts` before `-i`; `-c:v copy` (+ `-tag:v hvc1` for HEVC).
- Video-copy audio: transcode to AAC with the same conservative `aresample=async=1` rule as transcode.
- Input `-ss` uses the largest keyframe `<= StartSec` on the media timeline; remux jobs add a small forward pad on container time so ffmpeg lands on the intended keyframe.
- When `HLSVODJobOptions.SegmentList` is set, copy/remux jobs use **explicit** `-f segment -segment_times …` cuts rebased to the keyframe seek so mid-file restarts (`StartIndex > 0`) match the absolute keyframe grid from a from-zero job. Without `SegmentList`, legacy `-f hls` copy mode applies (from-zero only).

### Hardware encoders

Same structure; encoder/decoder/filter args come from `encode.Resolver` (NVENC, VAAPI, VideoToolbox, etc.). Golden argv tests in `ops/hls_vod_argv_test.go` lock the shared tail (`-output_ts_offset`, `-start_number`, HLS flags).

## Segment readiness

A segment is **ready** when `HLSVODSegmentReady` returns true:

1. `seg/NNNNN.m4s` exists and `seg/NNNNN.m4s.tmp` does **not** (ffmpeg `hls_flags temp_file` publish in progress).
2. The file is a publishable fMP4 fragment (`moof` + `tfdt`, monotonic decode times), at least `minHLSSegmentMediaBytes`, validated via bounded `moof` reads (`mp4.PublishableHLSSegmentReaderAt`).
3. **Explicit-cut copy** (`-f segment` + live CSV list): segment `k` is ready only when segment `k+1` appears in `.ffmpeg-vod-segments.csv` or the encode job has ended (multi-`moof` in-progress files are not served).

On ffmpeg builds where `temp_file` does not rename fMP4 segments atomically, a segment file can appear before the muxer finishes. Use `HLSVODSegmentReadyAfter(outDir, k)` only as a fallback; prefer moof completeness checks and waiting until segment `k` (not `k+1`) is publishable.

Partial files must not be visible to readers when `temp_file` works; hosts should not bypass the readiness helpers.

## Host restart guidance

- **One active job per file + transcode profile** (cache fingerprint), with optional future support for two concurrent forward jobs (TODO).
- Use `DecideHLSVODSegmentAction` / `HLSVODSession` (Jellyfin DynamicHls-style):
  - If `seg/k.m4s` is ready → **serve** (`HLSVODOpenReadySegment`).
  - Else if no running job, request `k` is behind the job start, or `k` is more than ~**24s of media** ahead of the encode head → **restart** at `StartIndex=k`, `StartSec=starts[k]` (copy jobs pass `SegmentList` for aligned cuts). Do not clear segments `< k` on restart.
  - Else → **wait** (poll outside the session lock; honor `context` cancel).
- Never cancel a job while `HLSVODSession` active serve refcounts are non-zero; on kill remove only the in-progress partial segment file.
- Idle timeout: cancel jobs after no segment requests for the profile (host policy).
- Serve `BuildVODPlaylist` output for the media playlist route; never expose `.ffmpeg-vod.m3u8` or continuous `ffmpeg.m3u8` EVENT playlists for this mode.
- Run ffprobe up front for duration, `start_time`, keyframes (copy), and pipeline selection.

## Library API (go-ffmpeg)

| API | Role |
|-----|------|
| `BuildVODSegmentList` / fixed / keyframe helpers | Segment timeline |
| `SegmentIndexForTime` | Map `currentTime` → index + snapped start |
| `BuildVODPlaylist` | Synthetic VOD m3u8 body |
| `BuildHLSVODFFmpegArgs` | Frozen argv |
| `StartHLSVODSegmentJob` | Run forward encode to disk |
| `DecideHLSVODSegmentAction` | Serve / wait / restart decision |
| `HLSVODSession` | Refcounted wait/restart loop |
| `HLSVODSegmentReady` / `HLSVODOpenReadySegment` | Readiness + paths |

## Legacy APIs (deprecated for on-demand file playback)

- `StartHLSContinuous` / EVENT `ffmpeg.m3u8` + `job.json` generations
- Per-request `HLSInitAndSegment` / `HLSSegmentMedia` with `mp4.AlignFragmentToMediaStart`

These remain for existing FileBrowser v1.4.0 consumers; new integrations should use Model A above.

## FileBrowser Quantum adoption (follow-up)

1. **Playlist route:** return `BuildVODPlaylist` for the media m3u8 (static per cache key).
2. **Segment handler:** `HLSVODSession` + readiness check, wait/restart rules, single job per profile.
3. **hlsAdapter:** drop generation offsets / EVENT polling; trust playlist-local times.
4. **Preflight:** probe duration, keyframes, pipeline; build `VODSegmentList` once per cache entry.
