# HLS playback

go-ffmpeg targets **browser MSE** playback (Plex/Jellyfin-style), not generic HLS authoring. You pass segment index, timeline offset, and a `VideoProfile`; the library owns fMP4 mux flags, `output_ts_offset`, post-encode `tfdt` alignment, and pipeline selection (remux / video-copy / full transcode). This is the deep ffmpeg knowledge most wrappers leave to you.

## On-demand segments

Independent ffmpeg invocations per segment index. Outputs fMP4 (`moof`+`mdat`) with:

- `empty_moov` for media-only fragments
- `output_ts_offset` for continuous timeline without bogus `#EXT-X-DISCONTINUITY`
- Post-encode `tfdt` alignment via `mp4` package

Use `HLSInitAndSegment` for segment 0 (init + media), `HLSSegmentMedia` for later indices.

## Continuous jobs

Single `ffmpeg -f hls` writing `init.m4s` and `seg/%05d.m4s` under a cache directory.

| Pacing | Use case |
|--------|----------|
| `HLSContinuousCacheFill` | Background prefetch at max speed |
| `HLSContinuousLivePaced` | Watch-while-transcode (~1× readrate) |
| `HLSContinuousRemuxSegmentDeletion` | Stream-copy + segment deletion |

Background aligner rebases segment `tfdt` to the HLS media timeline. Resume via `StartIndex` / `StartSec`.

## Helpers

Root package exports timeline builders, cache fingerprinting, and pipeline selection (remux / video-copy / transcode). See godoc for `BuildHLSSegmentParams`, `HLSCacheFingerprint`, etc.

## Harness

`test/hls/` generates fixtures, runs a benchmark matrix, and serves a report dashboard — developer tooling, not the library API. See [test/hls/README.md](../test/hls/README.md).
