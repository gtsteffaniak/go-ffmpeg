.PHONY: build test test-integration lint report

ifeq ($(OS),Windows_NT)
BIN := bin/go-ffmpeg.exe
else
BIN := bin/go-ffmpeg
endif

build:
	go build -o $(BIN) ./cmd/go-ffmpeg

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
	GOFFMPEG_SKIP_HW=1 go test -tags=integration ./... -run Integration -v

lint:
	go vet ./...
	@test -z "$$(gofmt -s -l .)"

format:
	gofmt -s -w .