package ops

import (
	"os"
	"strings"
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/encode"
)

func TestSpriteSheetFilterSoftware(t *testing.T) {
	t.Parallel()
	caps := &capabilities.Capabilities{
		Filters: map[string]bool{"scale": true, "tile": true, "select": true},
		CodecMatrix: map[capabilities.VideoCodec]capabilities.CodecSupport{
			capabilities.CodecH264: {
				DecodePreferred: capabilities.DecoderSelection{
					Decoder: "h264",
					Accel:   capabilities.AccelNone,
					Kind:    "software",
				},
			},
		},
		Decoders: map[string]capabilities.DecoderCapability{
			"h264": {Available: true, Compiled: true},
		},
	}
	layout := spriteLayout{Columns: 5, Rows: 4, TileWidth: 160, TileHeight: 90, TileCount: 20}
	filter, err := BuildSpriteSheetFilter(caps, encode.VideoDecodeProfile{Codec: capabilities.CodecH264}, layout, 5.0, true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(filter, "prev_selected_t") {
		t.Fatalf("unexpected select: %s", filter)
	}
	if !strings.Contains(filter, "tile=5x4") {
		t.Fatalf("missing tile: %s", filter)
	}
}

func TestWriteSpriteWebVTT(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := dir + "/thumbs.vtt"
	layout := spriteLayout{Columns: 2, Rows: 2, TileWidth: 100, TileHeight: 50, TileCount: 4, StartSec: 0}
	if err := writeSpriteWebVTT(path, "sprite.jpg", layout, 10, false); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "sprite.jpg#xywh=0,0,100,50") {
		t.Fatalf("missing first tile cue: %s", data)
	}
	if !strings.Contains(string(data), "sprite.jpg#xywh=100,0,100,50") {
		t.Fatalf("missing second tile cue: %s", data)
	}
}
