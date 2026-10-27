package ops

import (
	"strings"
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/encode"
)

func spriteTestCaps(filters ...string) *capabilities.Capabilities {
	f := map[string]bool{"scale": true, "tile": true, "select": true}
	for _, name := range filters {
		f[name] = true
	}
	return &capabilities.Capabilities{
		Filters: f,
		CodecMatrix: map[capabilities.VideoCodec]capabilities.CodecSupport{
			capabilities.CodecH264: {DecodePreferred: capabilities.DecoderSelection{
				Decoder: "h264", Accel: capabilities.AccelNone, Kind: "software",
			}},
		},
		Decoders: map[string]capabilities.DecoderCapability{
			"h264": {Available: true, Compiled: true},
		},
	}
}

func TestSpriteSheetFilterPad43Software(t *testing.T) {
	t.Parallel()
	caps := spriteTestCaps()
	layout := spriteLayout{Columns: 2, Rows: 2, TileWidth: 640, TileHeight: 480, TileCount: 4}
	filter, err := BuildSpriteSheetFilter(caps, encode.VideoDecodeProfile{Codec: capabilities.CodecH264}, layout, 5.0, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(filter, "pad=640:480:(ow-iw)/2:(oh-ih)/2") {
		t.Fatalf("missing pad: %s", filter)
	}
}

func TestSpriteHWScaleChainVAAPI(t *testing.T) {
	t.Parallel()
	caps := spriteTestCaps("scale_vaapi")
	caps.CodecMatrix[capabilities.CodecH264] = capabilities.CodecSupport{
		DecodePreferred: capabilities.DecoderSelection{
			Decoder: "hwaccel:vaapi:h264", Accel: capabilities.AccelVAAPI, Kind: "vaapi", SWCodec: "h264",
		},
	}
	caps.Decoders["hwaccel:vaapi:h264"] = capabilities.DecoderCapability{Available: true, Compiled: true, SWCodec: "h264"}
	layout := spriteLayout{TileWidth: 160, TileHeight: 90, Columns: 2, Rows: 2}
	filter, err := BuildSpriteSheetFilter(caps, encode.VideoDecodeProfile{Codec: capabilities.CodecH264}, layout, 2.0, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(filter, "scale_vaapi=w=160:h=90:force_original_aspect_ratio=decrease,hwdownload,format=nv12,pad=160:90") {
		t.Fatalf("unexpected vaapi chain: %s", filter)
	}
}

func TestSpriteHWScaleChainVulkan(t *testing.T) {
	t.Parallel()
	caps := spriteTestCaps("scale_vulkan")
	caps.CodecMatrix[capabilities.CodecH264] = capabilities.CodecSupport{
		DecodePreferred: capabilities.DecoderSelection{
			Decoder: "hwaccel:vulkan:h264", Accel: capabilities.AccelVulkan, Kind: "vulkan", SWCodec: "h264",
		},
	}
	caps.Decoders["hwaccel:vulkan:h264"] = capabilities.DecoderCapability{Available: true, Compiled: true, SWCodec: "h264"}
	layout := spriteLayout{TileWidth: 160, TileHeight: 90, Columns: 2, Rows: 2}
	filter, err := BuildSpriteSheetFilter(caps, encode.VideoDecodeProfile{Codec: capabilities.CodecH264}, layout, 2.0, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(filter, "hwdownload,format=nv12,scale=160:90") {
		t.Fatalf("expected vulkan hwdownload + software letterbox: %s", filter)
	}
	if strings.Contains(filter, "scale_vulkan=") {
		t.Fatalf("scale_vulkan must not use unsupported options: %s", filter)
	}
}

func TestSpriteHWScaleChainVideoToolbox(t *testing.T) {
	t.Parallel()
	caps := spriteTestCaps("scale_vt")
	caps.CodecMatrix[capabilities.CodecH264] = capabilities.CodecSupport{
		DecodePreferred: capabilities.DecoderSelection{
			Decoder: "hwaccel:videotoolbox:h264", Accel: capabilities.AccelVideoToolbox, Kind: "videotoolbox", SWCodec: "h264",
		},
	}
	caps.Decoders["hwaccel:videotoolbox:h264"] = capabilities.DecoderCapability{Available: true, Compiled: true, SWCodec: "h264"}
	layout := spriteLayout{TileWidth: 160, TileHeight: 90, Columns: 2, Rows: 2}
	filter, err := BuildSpriteSheetFilter(caps, encode.VideoDecodeProfile{Codec: capabilities.CodecH264}, layout, 2.0, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(filter, "scale_vt=w=160:h=90") || strings.Contains(filter, "hwdownload,format=nv12,scale_vt") {
		t.Fatalf("scale_vt must precede hwdownload: %s", filter)
	}
}

func TestSpriteHWScaleChainCUDA(t *testing.T) {
	t.Parallel()
	caps := spriteTestCaps("scale_cuda")
	caps.CodecMatrix[capabilities.CodecH264] = capabilities.CodecSupport{
		DecodePreferred: capabilities.DecoderSelection{
			Decoder: "h264_cuvid", Accel: capabilities.AccelNVENC, Kind: "nvenc",
		},
	}
	caps.Decoders["h264_cuvid"] = capabilities.DecoderCapability{Available: true, Compiled: true}
	layout := spriteLayout{TileWidth: 160, TileHeight: 90, Columns: 2, Rows: 2}
	filter, err := BuildSpriteSheetFilter(caps, encode.VideoDecodeProfile{Codec: capabilities.CodecH264}, layout, 2.0, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(filter, "scale_cuda=160:90") || strings.Contains(filter, "hwdownload,format=nv12,scale_cuda") {
		t.Fatalf("scale_cuda must precede hwdownload: %s", filter)
	}
}
