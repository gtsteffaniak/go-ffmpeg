.PHONY: build setup test test-race test-integration test-hls test-ffmpeg-matrix serve-report lint format report

ifeq ($(OS),Windows_NT)
BIN := bin/go-ffmpeg.exe
else
BIN := bin/go-ffmpeg
endif

SAMPLE := test/data/Big_Buck_Bunny_1080_10s_2MB.mp4
HLS_DIR := test/hls
HLS_BIN := $(HLS_DIR)/test-ffmpeg
HLS_SRCS := $(wildcard $(HLS_DIR)/*.go) $(HLS_DIR)/go.mod $(HLS_DIR)/go.sum
LIB_SRCS := go.mod go.sum $(wildcard *.go) \
	$(wildcard ops/*.go) $(wildcard mp4/*.go) $(wildcard encode/*.go) \
	$(wildcard capabilities/*.go) $(wildcard exec/*.go) $(wildcard probe/*.go) \
	$(wildcard platform/*.go) $(wildcard gtlogger/*.go)
REPORT_DIR := $(HLS_DIR)/report_site
FIXTURES_DIR := $(HLS_DIR)/.fixtures

REFERENCE ?= $(SAMPLE)
SEGMENTS ?= 0
FIXTURE_DURATION ?= 10
CONTINUOUS ?= true
REPORT_PORT ?= 8765
# Optional subset, e.g. FIXTURE_NAMES=h264_aac_mp4,hevc_aac_mp4 (default: all 21 fixtures)
FIXTURE_NAMES ?=

build:
	go build -o $(BIN) ./cmd/go-ffmpeg

$(HLS_BIN): $(HLS_SRCS) $(LIB_SRCS)
	go build -C $(HLS_DIR) -o test-ffmpeg .

# FFmpeg capability report (terminal only — not the HLS dashboard)
report: build
ifeq ($(OS),Windows_NT)
	powershell -NoProfile -ExecutionPolicy Bypass -File scripts/report-windows.ps1 -Binary $(BIN)
	@echo Report also saved to compatibility-report.txt
else
	./$(BIN) -color always | tee compatibility-report.txt
	@echo "Report also saved to compatibility-report.txt (view with: less -R compatibility-report.txt)"
endif

test:
	go test ./... -count=1

test-race:
	go test -race ./... -count=1

test-integration:
	@test -f "$(SAMPLE)" || (echo "missing sample video: $(SAMPLE)" >&2; exit 1)
	GOFFMPEG_SAMPLE_MP4="$(SAMPLE)" GOFFMPEG_SKIP_HW="$${GOFFMPEG_SKIP_HW:-1}" \
		go test -tags=integration ./... -run "Integration|ContinuousSoftware" -count=1

# Minimum fixtures for go test -tags=integration (encode/remux checks in test/hls).
HLS_INTEGRATION_FIXTURES := h264_aac_mp4,wmv2_wmapro_wmv
# Fixtures generated before go test: full FIXTURE_NAMES when set, else integration minimum.
HLS_TEST_FIXTURES := $(if $(strip $(FIXTURE_NAMES)),$(FIXTURE_NAMES),$(HLS_INTEGRATION_FIXTURES))

test-hls: $(HLS_BIN)
	@test -f "$(REFERENCE)" || (echo "missing reference video: $(REFERENCE)" >&2; exit 1)
	@echo "Generating fixtures for tests: $(HLS_TEST_FIXTURES)"
	$(HLS_BIN) generate-fixtures -reference "$(REFERENCE)" -out "$(FIXTURES_DIR)" -duration $(FIXTURE_DURATION) \
		-fixture-names "$(HLS_TEST_FIXTURES)"
	@echo "Unit tests (test/hls, no fixtures required)"
	go test -C $(HLS_DIR) -count=1 ./...
	@echo "Integration tests (test/hls, requires .fixtures)"
	go test -C $(HLS_DIR) -tags=integration -count=1 ./...
ifeq ($(strip $(FIXTURE_NAMES)),)
	$(HLS_BIN) generate-fixtures -reference "$(REFERENCE)" -out "$(FIXTURES_DIR)" -duration $(FIXTURE_DURATION)
	$(HLS_BIN) run -reference "$(REFERENCE)" -fixtures "$(FIXTURES_DIR)" -report "$(REPORT_DIR)" \
		-segments $(SEGMENTS) -duration $(FIXTURE_DURATION) -continuous=$(CONTINUOUS) -serve=false -skip-generate
else
	$(HLS_BIN) run -reference "$(REFERENCE)" -fixtures "$(FIXTURES_DIR)" -report "$(REPORT_DIR)" \
		-segments $(SEGMENTS) -duration $(FIXTURE_DURATION) -continuous=$(CONTINUOUS) -serve=false -skip-generate \
		-fixture-names "$(FIXTURE_NAMES)"
endif

serve-report: $(HLS_BIN)
	@test -f "$(REPORT_DIR)/data/report.json" || (echo "No report yet. Run: make test-hls" >&2; exit 1)
	$(HLS_BIN) serve-report -report "$(REPORT_DIR)" -port $(REPORT_PORT)

lint:
	go vet ./...
	go vet -C $(HLS_DIR) ./...
	@test -z "$$(gofmt -s -l . $(HLS_DIR))"

format:
	gofmt -s -w .
	gofmt -s -w $(HLS_DIR)

setup:
	bash scripts/setup.sh

# Run matrix gate tests for one gtstef/ffmpeg version in docker, e.g. make test-ffmpeg-version VERSION=8.1.2
test-ffmpeg-version:
	@test -n "$(VERSION)" || (echo "VERSION required, e.g. VERSION=8.1.2" >&2; exit 1)
	bash scripts/docker/run-version-tests.sh "$(VERSION)"

# Parallel matrix for all versions in docker/versions (same as pre-push hook / CI ffmpeg-matrix).
test-ffmpeg-matrix:
	bash scripts/docker/run-matrix-parallel.sh

test-integration-docker:
	bash scripts/docker/run-integration-tests.sh

test-hls-docker:
	bash scripts/docker/run-hls-tests.sh
