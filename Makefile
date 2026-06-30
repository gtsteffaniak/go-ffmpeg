.PHONY: build test integration-tests serve-results lint report format

SAMPLE := test/data/Big_Buck_Bunny_1080_10s_2MB.mp4
SEGMENTS ?= 3

ifeq ($(OS),Windows_NT)
BIN := bin/go-ffmpeg.exe
else
BIN := bin/go-ffmpeg
endif

build:
	go build -o $(BIN) ./cmd/go-ffmpeg

# FFmpeg capability report (terminal only — not the integration dashboard)
report: build
ifeq ($(OS),Windows_NT)
	powershell -NoProfile -ExecutionPolicy Bypass -File scripts/report-windows.ps1 -Binary $(BIN)
	@echo Report also saved to compatibility-report.txt
else
	./$(BIN) -color always | tee compatibility-report.txt
	@echo "Report also saved to compatibility-report.txt (view with: less -R compatibility-report.txt)"
endif

# Unit tests (fast; no integration tag)
test:
	go test ./... -count=1
	cd test/hls && go test -count=1 ./...

# Integration tests: ffmpeg, HLS matrix, writes test/hls/report_site/data/report.json
integration-tests:
	bash scripts/integration-tests.sh

# Serve the integration results dashboard (run after integration-tests)
serve-results:
	@if [ ! -f test/hls/report_site/data/report.json ]; then \
		echo "No results found. Run: make integration-tests" >&2; \
		exit 1; \
	fi
	cd test/hls && $(MAKE) serve-results

lint:
	go vet ./...
	@test -z "$$(gofmt -s -l .)"

format:
	gofmt -s -w .
