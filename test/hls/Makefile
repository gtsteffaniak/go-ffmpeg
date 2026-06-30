.PHONY: build check debug matrix all clean serve-hls playback-test \
	generate-results serve-results fixtures ci ci-software

SAMPLE ?= ../data/Big_Buck_Bunny_1080_10s_2MB.mp4
FILE ?= $(SAMPLE)
REFERENCE ?= $(SAMPLE)
SEGMENTS ?= 0
MODE ?= remux
PLAYBACK_SEGMENTS ?= 8
PLAYBACK_DURATION ?= 15
FIXTURE_DURATION ?= 10
REPORT_DIR ?= report_site

build:
	go build -o test-ffmpeg .

check: build
	./test-ffmpeg hls-check -file "$(FILE)" -segments $(SEGMENTS) -mode $(MODE)

debug: build
	./test-ffmpeg hls-check -file "$(FILE)" -segments 3 -mode $(MODE) -debug

fixtures: build
	./test-ffmpeg generate-fixtures -reference "$(REFERENCE)" -out .fixtures -duration $(FIXTURE_DURATION)

# Called by root make integration-tests — writes report_site/data/report.json
generate-results: build
	./test-ffmpeg run -reference "$(REFERENCE)" -fixtures .fixtures -report $(REPORT_DIR) \
		-segments $(SEGMENTS) -duration $(FIXTURE_DURATION) -serve=false

serve-results: build
	./test-ffmpeg serve-report -report $(REPORT_DIR)

matrix: build
	./test-ffmpeg matrix -reference "$(REFERENCE)" -fixtures .fixtures -segments $(SEGMENTS) -duration $(FIXTURE_DURATION)

serve-hls: build
	./test-ffmpeg serve-hls -file "$(FILE)" -segments $(PLAYBACK_SEGMENTS) -mode $(MODE)

playback-test: build
	cd playback && npm install --no-fund --no-audit 2>/dev/null || true
	cd playback && npx playwright install chromium 2>/dev/null || true
	./test-ffmpeg playback-test -file "$(FILE)" -segments $(PLAYBACK_SEGMENTS) -mode $(MODE) -duration $(PLAYBACK_DURATION)

all: check matrix

clean:
	rm -f test-ffmpeg report.json
	rm -rf .playback-cache

clean-all: clean
	rm -rf .fixtures .ci $(REPORT_DIR)/media $(REPORT_DIR)/data/report.json

# Software-only subset for GitHub Actions (fast; no report site required)
ci-software:
	bash scripts/ci-software-matrix.sh

ci: ci-software
