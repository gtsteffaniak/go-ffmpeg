// Package ffmpeg is a config-driven Go wrapper around the FFmpeg and FFprobe CLIs.
//
// Callers describe high-level tasks (probe, screenshot, transcode, HLS, timelapse,
// convert, subtitles) with typed options instead of raw argv. The library handles
// capability detection with seamless fallback, task-optimized ffmpeg flags,
// hardware-aware encoder selection, and version-safe options. HLS and transcode
// paths target Plex/Jellyfin-class browser streaming (fMP4 segments, timeline
// alignment, continuous cache jobs). Requires ffmpeg 5.0+ on the host.
//
// Install:
//
//	go get github.com/gtsteffaniak/go-ffmpeg
//
// Quick start:
//
//	svc, err := ffmpeg.New(ctx, ffmpeg.Config{FFmpegPath: "/usr/local/bin"})
//	info, err := svc.ProbeFile(ctx, "video.mp4")
//
// Capability detection runs on startup (or lazily on the first operation). A
// standalone reporter CLI is available:
//
//	go install github.com/gtsteffaniak/go-ffmpeg/cmd/go-ffmpeg@latest
//	go-ffmpeg -ffmpeg-path /path/to/ffmpeg
//
// See docs/ and CONTRIBUTING.md for HLS playback, hardware, and contributor guidelines.
package ffmpeg
