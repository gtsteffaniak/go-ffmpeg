# Contributing to go-ffmpeg

go-ffmpeg is a **task wrapper**, not an argv builder. New work should keep callers on typed options and `Service` methods while the library owns ffmpeg flags, capability gating, and hardware fallback. See the README "Why go-ffmpeg" section for the product goals.

## Repository layout

| Path | Purpose |
|------|---------|
| Root module | Public `package ffmpeg` + `Service` API |
| `capabilities/` | Startup detection, codec matrix, reports |
| `encode/` | Profile resolution, throttle, encoder args |
| `ops/` | High-level ffmpeg operations |
| `exec/`, `probe/`, `mp4/`, `platform/` | Process runner, ffprobe, fMP4 atoms, GPU |
| `cmd/go-ffmpeg` | Standalone capability reporter CLI |
| `test/hls/` | Separate module; HLS fixture harness and report site |

`make test` runs the root module. `make test-hls` runs the harness (fixtures + integration + optional full matrix).

## Adding a high-level operation

1. Options struct + implementation in `ops/`.
2. `Service` method on the root package (type aliases as needed).
3. Register `ops.Operation` with `Requirements` (encoders, filters, protocols, min profile).
4. Gate with `require()` → `ErrUnsupported` / `ProfileError` before spawning ffmpeg.
5. Service methods take a **concurrency slot** (`MaxConcurrent`); continuous jobs hold one slot until `Wait` completes.
6. Return `OperationError` with classified `Kind` via `wrapOp` for encode failures.
7. Version-gate new ffmpeg flags with `capabilities.FeatureFlags`.
8. Godoc on options; `Example*` test if user-facing.
9. Update [docs/operations.md](docs/operations.md) in the same change.

## Backwards compatibility

- Exported `package ffmpeg` types and methods are the semver contract.
- Prefer additive changes: new fields with zero-value = old behavior, new methods.
- Breaking changes: removing/renaming exports, changing field meaning, raising default `MinVersion`, changing HLS byte layout or cache fingerprint (bump `HLSCacheSchemaVersion`).
- Internal packages may evolve; do not break `Service` signatures in v1 without a major release.

## Versioning

- Go module: semantic versioning on `github.com/gtsteffaniak/go-ffmpeg` (v2 would use `/v2`).
- Tag releases `vX.Y.Z` after LICENSE and changelog exist.
- Document ffmpeg minimum changes in release notes.

## Tests

| Layer | Expectation |
|-------|-------------|
| Unit | ffmpeg required; `t.Fatalf` if missing |
| Integration (`-tags=integration`) | Real ffmpeg + sample video for media tests |
| Encode/HLS | Assert output meaning (timeline, format), not only `err == nil` |
| Race | `make test-race` must stay green |

## CI / matrix

- PR: lint (root + `test/hls`), unit `-race`, integration on full **8.1.1** docker image, HLS software matrix.
- **`ffmpeg-matrix`:** 5.1 / 6.1 / 7.1 / 8.1 — detection, version-gated throttle tests, integration subset (detect, duration, screenshot), CLI report.
- **`apt-ffmpeg`:** stripped distro detection and graceful unsupported ops.
- `GOFFMPEG_SKIP_HW=1` on GitHub-hosted runners; no GPU jobs without self-hosted runners.
- Do not commit `test/hls/report_site/media/` or generated fixtures.
- Add a matrix case when introducing a new version-gated ffmpeg flag.

## Style

- `gofmt -s`, `go vet`, no panics in library code.
- `exec.CommandContext` only — never shell out.
- Injected `Logger`; no global log state.
- Comments explain ffmpeg quirks (why), not obvious what.

## PR checklist

- [ ] Tests run (`make test` and integration/HLS if behavior changed)
- [ ] Additive vs breaking called out
- [ ] Operation registered and documented
- [ ] Version-gated flags noted if ffmpeg 5/6/7/8 behavior differs
