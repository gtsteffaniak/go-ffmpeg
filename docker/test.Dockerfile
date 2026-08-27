# Test runner: Go toolchain + ffmpeg/ffprobe copied from gtstef/ffmpeg (regular build, not -decode).
# Build: docker build -f docker/test.Dockerfile --build-arg FFMPEG_IMAGE=gtstef/ffmpeg:8.1.2 -t go-ffmpeg-test:8.1.2 .
ARG FFMPEG_IMAGE=gtstef/ffmpeg:8.1.2
FROM ${FFMPEG_IMAGE} AS ffmpeg

FROM golang:1.25-bookworm
COPY --from=ffmpeg [ "/ffmpeg", "/ffprobe", "/usr/local/bin/" ]

RUN apt-get update \
	&& apt-get install -y --no-install-recommends make ca-certificates \
	&& rm -rf /var/lib/apt/lists/*

ENV PATH="/usr/local/bin:${PATH}" \
	GOFFMPEG_FFMPEG_PATH=/usr/local/bin/ffmpeg \
	GOFFMPEG_FFPROBE_PATH=/usr/local/bin/ffprobe \
	GOFFMPEG_SKIP_HW=1 \
	CGO_ENABLED=0 \
	GOMODCACHE=/gomodcache \
	GOCACHE=/tmp/gocache \
	GOFLAGS=-buildvcs=false

# Pre-download modules at image build (repo mount at run time is read-only).
COPY go.mod go.sum /gomodsrc/
WORKDIR /gomodsrc
RUN go mod download

WORKDIR /src

RUN ffmpeg -version 2>&1 | sed -n '1p' && ffprobe -version 2>&1 | sed -n '1p'
