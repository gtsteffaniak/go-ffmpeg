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
| `examples/` | Runnable task demos (`go run ./examples/<name>`) |
| `test/hls/` | Separate module; HLS fixture harness and report site |

`make test` runs the root module. `make test-hls` runs the harness (fixtures + integration + optional full matrix).

## Adding a high-level operation

1. Options struct + implementation in `ops/`.
2. `Service` method on the root package (type aliases as needed).
3. Register `ops.Operation` with `Requirements` (encoders, filters, protocols, min profile).
4. Gate with `require()` → `ErrUnsupported` / `ProfileError` before spawning ffmpeg.
5. Service methods take a **concurrency slot** from the matching tier (`Config.Concurrency`); continuous jobs hold one slot until `Wait` completes.
6. Return `OperationError` with classified `Kind` via `wrapOp` for encode failures.
7. Version-gate new ffmpeg flags with `capabilities.FeatureFlags`.
8. Godoc on options; `Example*` test if user-facing.
9. Runnable program under `examples/<task>/` when the workflow is non-trivial.
10. Update [docs/operations.md](docs/operations.md) in the same change.

## Backwards compatibility

- Exported `package ffmpeg` types and methods are the semver contract.
- Prefer additive changes: new fields with zero-value = old behavior, new methods.
- Breaking changes: removing/renaming exports, changing field meaning, raising default `MinVersion`, changing HLS byte layout or cache fingerprint (bump `HLSCacheSchemaVersion`).
- Internal packages may evolve; do not break `Service` signatures in v1 without a major release.

## Versioning

- Go module: semantic versioning on `github.com/gtsteffaniak/go-ffmpeg` (v2 would use `/v2`).
- Tag releases `vX.Y.Z` after LICENSE and changelog exist.
- Document ffmpeg minimum changes in release notes.

## Pre-commit hooks

We use [pre-commit](https://pre-commit.com/) as the git hook runner. All hooks are **`language: system`** shell/Go scripts — no pip download on setup (works behind corporate TLS inspection).

### Install

```bash
make setup
```

This installs pre-commit (via Homebrew or pip if missing), registers the **pre-commit** git hook, and checks for **go**, **ffmpeg**, **ffprobe**, **docker**, and the sample video under `test/data/`.

### What runs on `git commit` (parallel where possible)

| Hook | Notes |
|------|-------|
| trailing-whitespace, end-of-file-fixer, check-merge-conflict | All staged files |
| gofmt, go vet, `go test ./... -short` | When `.go` files change; unit tests need **ffmpeg on PATH** or `GOFFMPEG_*_PATH` |
| **FFmpeg matrix** (docker) | All versions in `docker/versions` via `gtstef/ffmpeg` — same gates as CI `ffmpeg-matrix`. Requires **Docker** and `test/data/Big_Buck_Bunny_1080_10s_2MB.mp4`. Tests run **inside** the container (no binary extract to host). |

First matrix run pulls images and builds test containers; subsequent runs are faster.

HLS software matrix and `-race` unit tests are **CI-only** (or `make test-hls-docker` / `make test-race` manually).

### Manual runs

```bash
pre-commit run --all-files
make test-ffmpeg-matrix                    # parallel matrix
make test-ffmpeg-version VERSION=8.1.2   # single version
make test-integration-docker               # full integration on 8.1.2
make test-hls-docker                       # HLS harness on 8.1.2
GOFFMPEG_MATRIX_SKIP_MISSING=1 make test-ffmpeg-matrix   # skip unpublished images
SKIP=ffmpeg-matrix-docker git commit       # skip matrix only
```

Docker helpers: `scripts/docker/` (`docker/test.Dockerfile` copies ffmpeg from `gtstef/ffmpeg`, regular tag not `-decode`).

## Tests

| Layer | Expectation |
|-------|-------------|
| Unit | ffmpeg required; `t.Fatalf` if missing |
| Integration (`-tags=integration`) | Real ffmpeg + sample video for media tests |
| Encode/HLS | Assert output meaning (timeline, format), not only `err == nil` |
| Race | `make test-race` must stay green |

## CI / matrix

- PR: lint, unit `-race`, integration + HLS in **docker** (`gtstef/ffmpeg:8.1.2`), **`ffmpeg-matrix`** for every version in `docker/versions`.
- Matrix tests run inside `docker/test.Dockerfile` (no BtbN tarballs or musl binary extract).
- **`apt-ffmpeg`:** Ubuntu apt `ffmpeg` on the host — `TestDetect` / `TestFeatureFlags` / `TestAppendReadrate` plus `go-ffmpeg -skip-hw-tests` JSON report (smoke that distro builds work with detection).
- `GOFFMPEG_SKIP_HW=1` on GitHub-hosted runners.
- Do not commit `test/hls/report_site/media/` or generated fixtures.
- Add a version to `docker/versions` when a new `gtstef/ffmpeg` tag ships.

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
