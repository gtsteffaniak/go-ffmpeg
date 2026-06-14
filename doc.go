// Package ffmpeg provides a tested wrapper around FFmpeg and FFprobe with
// startup capability detection, config-driven operations, and a long-lived Service.
//
// A standalone compatibility reporter is available as cmd/go-ffmpeg:
//
//	go install github.com/gtsteffaniak/go-ffmpeg/cmd/go-ffmpeg@latest
//	go-ffmpeg -ffmpeg-path /path/to/ffmpeg
package ffmpeg
