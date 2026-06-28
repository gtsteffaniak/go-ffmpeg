package encode

import (
	"fmt"
	"strings"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/probe"
)

// HLSPreset names browser-oriented HLS transcode quality levels (library-neutral).
type HLSPreset string

const (
	HLSPresetQuality     HLSPreset = "quality"
	HLSPresetLowLatency  HLSPreset = "low-latency"
	HLSPresetConstrained HLSPreset = "constrained"
)

const hlsConstrainedMaxHeight = 720

// NormalizeHLSPreset maps aliases to canonical preset names.
func NormalizeHLSPreset(raw HLSPreset) HLSPreset {
	switch strings.ToLower(strings.TrimSpace(string(raw))) {
	case string(HLSPresetLowLatency), "optimized", "low_latency":
		return HLSPresetLowLatency
	case string(HLSPresetConstrained), "datasaver", "data-saver", "data_saver":
		return HLSPresetConstrained
	default:
		return HLSPresetQuality
	}
}

// HLSDecodeProfileForOnDemand selects input decode for short on-demand HLS segments.
func HLSDecodeProfileForOnDemand(info probe.StreamInfo) VideoDecodeProfile {
	if !isKnownHLSInputVideoCodec(info.VideoCodec) {
		return VideoDecodeProfile{ForceSoftware: true}
	}
	return OnDemandHLSDecodeProfile(VideoDecodeProfile{
		Codec: probeVideoCodec(info.VideoCodec),
	})
}

// HLSVideoProfile selects output encode settings for an HLS transcode preset.
func HLSVideoProfile(info probe.StreamInfo, preset HLSPreset, maxHeight int) VideoProfile {
	switch NormalizeHLSPreset(preset) {
	case HLSPresetLowLatency:
		return VideoProfile{
			Codec:   CodecH264,
			Quality: PresetVeryfast,
			Bitrate: hlsLowLatencyBitrateConfig(info, maxHeight),
		}
	case HLSPresetConstrained:
		return VideoProfile{
			Codec:   CodecH264,
			Quality: PresetVeryfast,
			Bitrate: hlsConstrainedBitrateConfig(info, maxHeight),
		}
	default:
		return VideoProfile{
			Codec:   CodecH264,
			Quality: PresetMedium,
			Bitrate: hlsQualityBitrateConfig(info, maxHeight),
		}
	}
}

func isKnownHLSInputVideoCodec(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "h264", "avc", "avc1", "hevc", "h265", "vp9", "av1":
		return true
	default:
		return false
	}
}

func probeVideoCodec(name string) capabilities.VideoCodec {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "h264", "avc", "avc1":
		return capabilities.CodecH264
	case "hevc", "h265":
		return capabilities.CodecHEVC
	case "vp9":
		return capabilities.CodecVP9
	case "av1":
		return capabilities.CodecAV1
	default:
		return capabilities.CodecH264
	}
}

func hlsOutputHeight(info probe.StreamInfo, preset HLSPreset, maxHeight int) int {
	maxH := maxHeight
	if NormalizeHLSPreset(preset) == HLSPresetConstrained {
		if maxH <= 0 || maxH > hlsConstrainedMaxHeight {
			maxH = hlsConstrainedMaxHeight
		}
	}
	if info.Height <= 0 {
		if maxH > 0 {
			return maxH
		}
		return 720
	}
	if maxH > 0 && info.Height > maxH {
		return maxH
	}
	return info.Height
}

func hlsTargetVideoKbps(info probe.StreamInfo, preset HLSPreset, maxHeight int) int {
	outHeight := hlsOutputHeight(info, preset, maxHeight)

	baseline := 1200
	switch {
	case outHeight >= 1080:
		baseline = 5000
	case outHeight >= 720:
		baseline = 3500
	case outHeight >= 480:
		baseline = 2000
	}

	target := baseline
	if info.VideoBitrate > 0 {
		sourceKbps := info.VideoBitrate / 1000
		if info.Height > 0 && outHeight > 0 && outHeight < info.Height {
			scale := float64(outHeight) / float64(info.Height)
			sourceKbps = int(float64(sourceKbps) * scale * scale)
		}
		target = sourceKbps
	}
	if target < baseline {
		target = baseline
	}

	const minKbps = 1500
	const maxKbps = 12000
	if target < minKbps {
		target = minKbps
	}
	if target > maxKbps {
		target = maxKbps
	}
	return target
}

func hlsQualityBitrateConfig(info probe.StreamInfo, maxHeight int) BitrateConfig {
	targetKbps := hlsTargetVideoKbps(info, HLSPresetQuality, maxHeight)
	return BitrateConfig{
		Target:  fmt.Sprintf("%dk", targetKbps),
		Min:     fmt.Sprintf("%dk", targetKbps/2),
		Max:     fmt.Sprintf("%dk", targetKbps*3/2),
		BufSize: fmt.Sprintf("%dk", targetKbps*2),
	}
}

func hlsLowLatencyBitrateConfig(info probe.StreamInfo, maxHeight int) BitrateConfig {
	targetKbps := hlsTargetVideoKbps(info, HLSPresetLowLatency, maxHeight)
	capKbps := targetKbps * 75 / 100
	const minCapKbps = 1000
	if capKbps < minCapKbps {
		capKbps = minCapKbps
	}
	return BitrateConfig{
		Target:  fmt.Sprintf("%dk", capKbps),
		Max:     fmt.Sprintf("%dk", capKbps),
		BufSize: fmt.Sprintf("%dk", capKbps*2),
	}
}

func hlsConstrainedBitrateConfig(info probe.StreamInfo, maxHeight int) BitrateConfig {
	outHeight := hlsOutputHeight(info, HLSPresetConstrained, maxHeight)
	capKbps := hlsConstrainedCapKbps(outHeight)
	targetKbps := capKbps
	if info.VideoBitrate > 0 {
		sourceKbps := info.VideoBitrate / 1000
		if info.Height > 0 && outHeight > 0 && outHeight < info.Height {
			scale := float64(outHeight) / float64(info.Height)
			sourceKbps = int(float64(sourceKbps) * scale * scale)
		}
		if sourceKbps < targetKbps {
			targetKbps = sourceKbps
		}
	}
	const minKbps = 350
	if targetKbps < minKbps {
		targetKbps = minKbps
	}
	if targetKbps > capKbps {
		targetKbps = capKbps
	}
	return BitrateConfig{
		Target:  fmt.Sprintf("%dk", targetKbps),
		Max:     fmt.Sprintf("%dk", targetKbps),
		BufSize: fmt.Sprintf("%dk", targetKbps*2),
	}
}

// hlsConstrainedCapKbps returns the max video bitrate for data-saver by output height.
func hlsConstrainedCapKbps(outHeight int) int {
	switch {
	case outHeight >= 720:
		return 800
	case outHeight >= 480:
		return 550
	default:
		return 400
	}
}
