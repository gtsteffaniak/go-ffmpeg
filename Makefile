.PHONY: build test test-integration lint report

build:
	go build -o bin/go-ffmpeg ./cmd/go-ffmpeg

report: build
	./bin/go-ffmpeg -color always | tee compatibility-report.txt
	@echo "Report also saved to compatibility-report.txt (view with: less -R compatibility-report.txt)"

test:
	go test ./...

test-integration:
	GOFFMPEG_SKIP_HW=1 go test -tags=integration ./... -run Integration -v

lint:
	go vet ./...
	@test -z "$$(gofmt -s -l .)"
