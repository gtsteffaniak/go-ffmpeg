# HLS playback

go-ffmpeg targets **browser MSE** playback (Plex/Jellyfin-style), not generic HLS authoring.

**Recommended for on-demand file playback:** [Model A VOD contract](hls-vod-contract.md) — synthetic full-length `#EXT-X-PLAYLIST-TYPE:VOD` playlists, `StartHLSVODSegmentJob`, playlist-local `output_ts_offset`, no EVENT playlist dependency.

The sections below describe **legacy** APIs still used by existing integrations (e.g. FileBrowser v1.4.0). Prefer Model A for new work.

## On-demand segments (legacy per-request API)

Independent ffmpeg invocations per segment index. Outputs fMP4 (`moof`+`mdat`) with:

- `empty_moov` for media-only fragments
- `output_ts_offset` for continuous timeline without bogus `#EXT-X-DISCONTINUITY`
- Post-encode `tfdt` alignment via `mp4` package

Use `HLSInitAndSegment` for segment 0 (init + media), `HLSSegmentMedia` for later indices.

## Continuous jobs (legacy EVENT playlist)

Single `ffmpeg -f hls` writing `init.m4s` and `seg/%05d.m4s` under a cache directory. Prefer [VOD Model A](hls-vod-contract.md) for on-demand file playback instead of serving growing EVENT playlists.

| Pacing | Use case |
|--------|----------|
| `HLSContinuousCacheFill` | Background prefetch at max speed |
| `HLSContinuousLivePaced` | Watch-while-transcode (~1× readrate, ~20s initial burst; ffmpeg 8+ default catchup) |
| `HLSContinuousRemuxSegmentDeletion` | Stream-copy + segment deletion |

Background aligner rebases segment `tfdt` onto the **full-file decode timeline** (segment 0 at file head; seek jobs at the snapped `StartSec` + cumulative `#EXTINF`). Resume via `StartIndex` / `StartSec`. Consumers can read `job.json` `startSec` for the generation anchor; `<video>.currentTime` tracks absolute media time without subtracting an offset.

## Helpers

Root package exports timeline builders, cache fingerprinting, and pipeline selection (remux / video-copy / transcode). See godoc for `BuildHLSSegmentParams`, `HLSCacheFingerprint`, etc.

## Harness

`test/hls/` generates fixtures, runs a benchmark matrix, and serves a report dashboard — developer tooling, not the library API. See [test/hls/README.md](../test/hls/README.md).
