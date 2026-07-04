.PHONY: build test test-integration test-hls serve-report lint format report

ifeq ($(OS),Windows_NT)
BIN := bin/go-ffmpeg.exe
else
BIN := bin/go-ffmpeg
endif

SAMPLE := test/data/Big_Buck_Bunny_1080_10s_2MB.mp4
HLS_DIR := test/hls
HLS_BIN := $(HLS_DIR)/test-ffmpeg
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

$(HLS_BIN):
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

test-integration:
	@test -f "$(SAMPLE)" || (echo "missing sample video: $(SAMPLE)" >&2; exit 1)
	GOFFMPEG_SAMPLE_MP4="$(SAMPLE)" GOFFMPEG_SKIP_HW="$${GOFFMPEG_SKIP_HW:-1}" \
		go test -tags=integration ./... -run Integration -count=1

test-hls: $(HLS_BIN)
	go test -C $(HLS_DIR) -count=1 ./...
	$(HLS_BIN) run -reference "$(REFERENCE)" -fixtures "$(FIXTURES_DIR)" -report "$(REPORT_DIR)" \
		-segments $(SEGMENTS) -duration $(FIXTURE_DURATION) -continuous=$(CONTINUOUS) -serve=false \
		$(if $(strip $(FIXTURE_NAMES)),-fixture-names "$(FIXTURE_NAMES)",-fixture-names "")

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
