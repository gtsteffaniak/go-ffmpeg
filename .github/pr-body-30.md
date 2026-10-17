## Summary

- Publish continuous HLS `seg/*.m4s` with `tfdt` on the **full-file decode timeline** (`StartSec` + cumulative `#EXTINF`) instead of a generation-local `0 … producedSec` axis.
- Updates validation (`ValidateContinuousHLSOutput`), the `test/hls` continuous-check harness, unit tests, and `docs/hls.md`.
- Clarifies `HLSContinuousJob.StartSec()` godoc: published fragments are aligned for native `<video>.currentTime` without client-side offset subtraction.

Builds on [v1.3.0](https://github.com/gtsteffaniak/go-ffmpeg/releases/tag/v1.3.0) self-serve publish (`index.m3u8` + `job.json`).

## Problem

After PR #29, seek generations rebased fragments to **playlist-local** time (first listed segment at `tfdt ≈ 0`). `job.json` recorded the true anchor (`startSec`, e.g. 1887s), but MSE/hls.js saw a **0-based tail playlist**. Integrators (filebrowser, Plyr, `hlsAdapter.js`) had to bridge that gap manually:

- `hls.startLoad(requestedSec - job.startSec)`
- UI math to map `video.currentTime` back to full-file position

That contract is easy to get wrong and shows up as scrubber jumps, stalls on `startLoad(fullSec)`, or shrunk duration.

## Solution

`publishContinuousReadySegments` now aligns using `continuousPlaylistAbsoluteMediaStarts(playlist, startSec)`:

```
absoluteMediaStart = generationStartSec + cumulativeEXTINFWithinPlaylist
```

For a job starting at file head (`startSec == 0`), behavior is unchanged. For mid-file seeks, segment 384 lands at ~1887s decode time, not 0.

## Integrator contract (after merge)

| Topic | Guidance |
|-------|----------|
| **Display time** | Use `video.currentTime` directly once data is buffered; no `+ job.startSec` offset. |
| **Scrubber duration** | Still use probed **full** `durationSec` from the session while transcoding; `video.duration` may only reflect the buffered tail until the generation completes. |
| **hls.js load** | Prefer `loadSource(index.m3u8)` / default start (`startLoad(-1)`). Avoid `startLoad(fullFileSec)` unless the buffer actually spans that time. |
| **Out-of-range seeks** | Cancel and respawn a generation when the user seeks outside `[job.startSec, job.startSec + producedSec]`; `job.json` `startSec` remains the respawn anchor. |

## Files changed

- `ops/hls_continuous_align.go` — absolute media starts + publish alignment
- `ops/hls_continuous.go` — consumer-facing godoc
- `ops/continuous_validate.go` — expect full-file `tfdt` in validation
- `ops/hls_continuous_align_test.go` — seek publish expectations (1887s, not 0)
- `test/hls/continuous_check.go` — harness parity
- `docs/hls.md` — timeline documentation

## Test plan

- [x] `go test ./ops/ -short`
- [ ] `make check-all` / continuous HLS CI matrix (FFmpeg 5.1–9.x, resume_mid fixtures)
- [ ] Optional: `test/hls/playback` smoke on a `resume_mid` generation — confirm `currentTime` ≈ `job.startSec` after first fragment buffers

## Related

- Release baseline: v1.3.0 (`5bcdb2d`, PR #29)
- Follow-up idea (not in this PR): optional `durationSec` in `job.json` from probe for single-field session APIs
