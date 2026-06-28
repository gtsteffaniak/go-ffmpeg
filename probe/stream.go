package probe

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	ffexec "github.com/gtsteffaniak/go-ffmpeg/exec"
)

// StreamType identifies input stream protocols.
type StreamType string

const (
	StreamFile StreamType = "file"
	StreamRTSP StreamType = "rtsp"
	StreamHLS  StreamType = "hls"
	StreamHTTP StreamType = "http"
)

// ProbeStreamOptions configures stream probing.
type ProbeStreamOptions struct {
	URL        string
	StreamType StreamType
	Timeout    time.Duration
}

// StreamInfo holds ffprobe results for a stream.
type StreamInfo struct {
	IsValid      bool    `json:"isValid"`
	HasVideo     bool    `json:"hasVideo"`
	HasAudio     bool    `json:"hasAudio"`
	VideoCodec   string  `json:"videoCodec,omitempty"`
	AudioCodec   string  `json:"audioCodec,omitempty"`
	VideoBitrate int     `json:"videoBitrate,omitempty"`
	Width        int     `json:"width,omitempty"`
	Height       int     `json:"height,omitempty"`
	Duration     float64 `json:"duration,omitempty"`
	FormatName   string  `json:"formatName,omitempty"`
	Message      string  `json:"message,omitempty"`
}

// ProbeStream runs ffprobe against a URL or file.
func ProbeStream(ctx context.Context, runner *ffexec.Runner, opts ProbeStreamOptions) (StreamInfo, error) {
	info := StreamInfo{Message: "probe failed or stream invalid"}
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	pctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	args := []string{
		"-v", "info",
		"-print_format", "json",
		"-show_streams",
		"-show_format",
	}
	switch strings.ToLower(string(opts.StreamType)) {
	case "rtsp":
		args = append(args,
			"-rtsp_transport", "tcp",
			"-analyzeduration", "10000000",
			"-probesize", "50000000",
			"-timeout", "10000000",
		)
	case "hls", "http", "https":
		args = append(args, "-rw_timeout", "5000000")
	}
	args = append(args, opts.URL)

	res, err := runner.RunFFprobe(pctx, args...)
	if pctx.Err() == context.DeadlineExceeded {
		info.Message = "probe timeout"
		return info, fmt.Errorf("probe timeout: %w", err)
	}
	if err != nil {
		info.Message = res.Stderr
		if info.Message == "" {
			info.Message = err.Error()
		}
		return info, err
	}

	var result struct {
		Streams []probeStreamEntry `json:"streams"`
		Format  struct {
			FormatName string `json:"format_name"`
			BitRate    string `json:"bit_rate"`
			Duration   string `json:"duration"`
		} `json:"format"`
	}
	if err := json.Unmarshal([]byte(res.Stdout), &result); err != nil {
		info.Message = err.Error()
		return info, err
	}
	if len(result.Streams) == 0 {
		info.Message = "no streams found"
		return info, fmt.Errorf("no streams found")
	}

	info.IsValid = true
	info.FormatName = result.Format.FormatName
	if d, err := strconv.ParseFloat(result.Format.Duration, 64); err == nil {
		info.Duration = d
	}

	hasVideoBitrate := applyProbeStreams(&info, result.Streams)
	if info.HasVideo && !hasVideoBitrate && result.Format.BitRate != "" {
		if br, err := strconv.Atoi(result.Format.BitRate); err == nil {
			info.VideoBitrate = br
		}
	}
	if info.IsValid {
		info.Message = ""
	}
	return info, nil
}

type probeStreamEntry struct {
	CodecType    string `json:"codec_type"`
	CodecName    string `json:"codec_name"`
	BitRate      string `json:"bit_rate"`
	AvgFrameRate string `json:"avg_frame_rate"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
}

// applyProbeStreams records the first video and first audio stream, matching ffmpeg -map 0:v:0 / 0:a:0.
func applyProbeStreams(info *StreamInfo, streams []probeStreamEntry) (hasVideoBitrate bool) {
	for _, s := range streams {
		switch s.CodecType {
		case "video":
			if info.HasVideo {
				continue
			}
			info.HasVideo = true
			info.VideoCodec = s.CodecName
			info.Width = s.Width
			info.Height = s.Height
			if s.BitRate != "" {
				if br, err := strconv.Atoi(s.BitRate); err == nil && br > 0 {
					info.VideoBitrate = br
					hasVideoBitrate = true
				}
			}
		case "audio":
			info.HasAudio = true
			if info.AudioCodec == "" {
				info.AudioCodec = s.CodecName
			}
		}
	}
	return hasVideoBitrate
}

// GetMediaDuration returns media duration in seconds.
func GetMediaDuration(ctx context.Context, runner *ffexec.Runner, path string) (float64, error) {
	res, err := runner.RunFFprobe(ctx,
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		path,
	)
	if err != nil {
		return 0, err
	}
	durStr := strings.TrimSpace(res.Stdout)
	if durStr == "" || durStr == "N/A" {
		return 0, fmt.Errorf("duration unavailable")
	}
	dur, err := strconv.ParseFloat(durStr, 64)
	if err != nil || dur <= 0 {
		return 0, fmt.Errorf("invalid duration: %s", durStr)
	}
	return dur, nil
}

// GetImageDimensions returns width and height of the first video stream.
func GetImageDimensions(ctx context.Context, runner *ffexec.Runner, path string) (width, height int, err error) {
	res, err := runner.RunFFprobe(ctx,
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height",
		"-of", "csv=p=0",
		path,
	)
	if err != nil {
		return 0, 0, err
	}
	parts := strings.Split(strings.TrimSpace(res.Stdout), ",")
	if len(parts) < 2 {
		return 0, 0, fmt.Errorf("unexpected ffprobe output: %s", res.Stdout)
	}
	width, err = strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, 0, err
	}
	height, err = strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return 0, 0, err
	}
	return width, height, nil
}

// SubtitleTrack describes an embedded subtitle stream.
type SubtitleTrack struct {
	Index    int    `json:"index"`
	Codec    string `json:"codec"`
	Language string `json:"language,omitempty"`
	Title    string `json:"title,omitempty"`
}

// DetectEmbeddedSubtitles lists embedded subtitle streams.
func DetectEmbeddedSubtitles(ctx context.Context, runner *ffexec.Runner, path string) ([]SubtitleTrack, error) {
	res, err := runner.RunFFprobe(ctx,
		"-v", "quiet",
		"-print_format", "json",
		"-show_streams",
		path,
	)
	if err != nil {
		return nil, err
	}
	var output struct {
		Streams []struct {
			Index     int               `json:"index"`
			CodecType string            `json:"codec_type"`
			CodecName string            `json:"codec_name"`
			Tags      map[string]string `json:"tags"`
		} `json:"streams"`
	}
	if err := json.Unmarshal([]byte(res.Stdout), &output); err != nil {
		return nil, err
	}
	var tracks []SubtitleTrack
	for _, s := range output.Streams {
		if s.CodecType != "subtitle" {
			continue
		}
		t := SubtitleTrack{Index: s.Index, Codec: s.CodecName}
		if s.Tags != nil {
			t.Language = s.Tags["language"]
			t.Title = s.Tags["title"]
		}
		tracks = append(tracks, t)
	}
	return tracks, nil
}

// ExtractSubtitle extracts a subtitle stream to WebVTT text.
func ExtractSubtitle(ctx context.Context, runner *ffexec.Runner, path string, streamIndex int) (string, error) {
	res, err := runner.RunFFmpeg(ctx,
		"-i", path,
		"-map", fmt.Sprintf("0:%d", streamIndex),
		"-c:s", "webvtt",
		"-f", "webvtt",
		"-",
	)
	if err != nil {
		return "", err
	}
	return res.Stdout, nil
}
