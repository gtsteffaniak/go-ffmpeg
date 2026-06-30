package capabilities

import (
	"time"

	"github.com/gtsteffaniak/go-ffmpeg/platform"
)

// AccelType identifies a hardware acceleration backend.
type AccelType string

const (
	AccelNVENC        AccelType = "nvenc"
	AccelAMF          AccelType = "amf"
	AccelQSV          AccelType = "qsv"
	AccelVAAPI        AccelType = "vaapi"
	AccelD3D12        AccelType = "d3d12"
	AccelVideoToolbox AccelType = "videotoolbox"
	AccelNone         AccelType = "none"
)

// VideoCodec identifies a logical output video codec family.
type VideoCodec string

const (
	CodecH264 VideoCodec = "h264"
	CodecAV1  VideoCodec = "av1"
	CodecVP9  VideoCodec = "vp9"
	CodecHEVC VideoCodec = "hevc"
)

// DefaultHierarchy returns Linux-standard order when platform is unknown.
func DefaultHierarchy() []AccelType {
	return []AccelType{AccelNVENC, AccelVAAPI, AccelQSV, AccelAMF}
}

// AllEncoderAccelTypes lists every hardware backend the library understands.
func AllEncoderAccelTypes() []AccelType {
	return []AccelType{AccelNVENC, AccelVAAPI, AccelQSV, AccelAMF, AccelD3D12, AccelVideoToolbox}
}

// HierarchyForPlatform returns encoder preference order for the detected host.
// Software encoding is chosen automatically when no hardware path is available.
//
//   - Windows:     NVENC → QSV → AMF
//   - Linux:       NVENC → VAAPI → QSV → AMF
//   - WSL2:        NVENC → D3D12 → VAAPI → QSV
//   - macOS:       VideoToolbox only
func HierarchyForPlatform(plat PlatformInfo) []AccelType {
	switch plat.OS {
	case "darwin":
		return []AccelType{AccelVideoToolbox}
	case "windows":
		return []AccelType{AccelNVENC, AccelQSV, AccelAMF}
	}
	if plat.WSL {
		return []AccelType{AccelNVENC, AccelD3D12, AccelVAAPI, AccelQSV}
	}
	return []AccelType{AccelNVENC, AccelVAAPI, AccelQSV, AccelAMF}
}

// BuildProfile describes the inferred FFmpeg build type.
type BuildProfile string

const (
	BuildFull       BuildProfile = "full"
	BuildDecodeOnly BuildProfile = "decode-only"
	BuildUnknown    BuildProfile = "unknown"
)

// BuildConfig holds parsed configure flags from ffmpeg -version.
type BuildConfig struct {
	Flags    []string
	LibFlags []string
	RawLine  string
}

// DecoderCapability describes a single decoder's availability.
type DecoderCapability struct {
	Name      string     `json:"name"`
	Codec     VideoCodec `json:"codec,omitempty"`
	Compiled  bool       `json:"compiled"`
	Tested    bool       `json:"tested"`
	Available bool       `json:"available"`
	Kind      string     `json:"kind"`
	HWAccel   string     `json:"hwAccel,omitempty"`
	SWCodec   string     `json:"swCodec,omitempty"`
	TestError string     `json:"testError,omitempty"`
}

// DecoderSelection describes the preferred decoder for a codec family.
type DecoderSelection struct {
	Decoder  string    `json:"decoder"`
	Accel    AccelType `json:"accel"`
	Kind     string    `json:"kind"`
	Fallback string    `json:"fallback,omitempty"`
	HWAccel  string    `json:"hwAccel,omitempty"`
	SWCodec  string    `json:"swCodec,omitempty"`
}

// EncoderCapability describes a single encoder's availability.
type EncoderCapability struct {
	Name      string `json:"name"`
	Compiled  bool   `json:"compiled"`
	Tested    bool   `json:"tested"`
	Available bool   `json:"available"`
	Kind      string `json:"kind"`
	TestError string `json:"testError,omitempty"`
}

// HWAccelCapability describes a hardware acceleration method.
type HWAccelCapability struct {
	Name      string `json:"name"`
	Compiled  bool   `json:"compiled"`
	Available bool   `json:"available"`
}

// PlatformInfo records host driver/device gate results.
type PlatformInfo = platform.Info
type EncoderSelection struct {
	Encoder  string    `json:"encoder"`
	Accel    AccelType `json:"accel"`
	Kind     string    `json:"kind"`
	Fallback string    `json:"fallback,omitempty"`
}

// CodecSupport summarizes encode/decode options for a codec family.
type CodecSupport struct {
	Software        []string             `json:"software"`
	SoftwareDecode  []string             `json:"softwareDecode"`
	Hardware        map[AccelType]string `json:"hardware"`
	HardwareDecode  map[AccelType]string `json:"hardwareDecode"`
	Preferred       EncoderSelection     `json:"preferred"`
	DecodePreferred DecoderSelection     `json:"decodePreferred"`
}

// Capabilities is the full capability matrix produced by Detect.
type Capabilities struct {
	FFmpegPath     string                       `json:"ffmpegPath"`
	FFprobePath    string                       `json:"ffprobePath"`
	FFmpegVersion  string                       `json:"ffmpegVersion"`
	FFprobeVersion string                       `json:"ffprobeVersion"`
	BuildConfig    BuildConfig                  `json:"buildConfig"`
	BuildProfile   BuildProfile                 `json:"buildProfile"`
	Encoders       map[string]EncoderCapability `json:"encoders"`
	Decoders       map[string]DecoderCapability `json:"decoders"`
	Filters        map[string]bool              `json:"filters"`
	HWAccels       map[string]HWAccelCapability `json:"hwaccels"`
	Protocols      map[string]bool              `json:"protocols"`
	Platform       PlatformInfo                 `json:"platform"`
	CodecMatrix    map[VideoCodec]CodecSupport  `json:"codecMatrix"`
	EncodeOptions  []EncodeOption               `json:"encodeOptions"`
	DecodeOptions  []DecodeOption               `json:"decodeOptions"`
	EnabledOps     []string                     `json:"enabledOps"`
	DisabledOps    map[string][]string          `json:"disabledOps"`
	FeatureFlags     FeatureFlags                 `json:"featureFlags"`
	GeneratedAt      time.Time                    `json:"generatedAt"`
	EncoderHierarchy []AccelType                  `json:"encoderHierarchy,omitempty"`
	SelectedGPU      *platform.GPUChoice          `json:"selectedGpu,omitempty"`
}

// NewCapabilities returns an empty capability matrix.
func NewCapabilities() *Capabilities {
	return &Capabilities{
		Encoders:    make(map[string]EncoderCapability),
		Decoders:    make(map[string]DecoderCapability),
		Filters:     make(map[string]bool),
		HWAccels:    make(map[string]HWAccelCapability),
		Protocols:   make(map[string]bool),
		CodecMatrix: make(map[VideoCodec]CodecSupport),
		DisabledOps: make(map[string][]string),
		Platform:    PlatformInfo{Details: make(map[string]string)},
	}
}

// DecoderAvailable reports whether a decoder passed detection.
func (c *Capabilities) DecoderAvailable(name string) bool {
	if dec, ok := c.Decoders[name]; ok {
		return dec.Available
	}
	return false
}

// CodecEncodeAvailable reports whether hardware/software encode works for codec+accel.
func (c *Capabilities) CodecEncodeAvailable(codec VideoCodec, accel AccelType) bool {
	if accel == AccelNone {
		for _, sw := range SoftwareFallback(codec) {
			if c.EncoderAvailable(sw) {
				return true
			}
		}
		return false
	}
	if enc := CodecEncoderMap(codec, accel); enc != "" {
		return c.EncoderAvailable(enc)
	}
	return false
}

// CodecDecodeAvailable reports whether hardware/software decode works for codec+accel.
func (c *Capabilities) CodecDecodeAvailable(codec VideoCodec, accel AccelType) bool {
	if accel == AccelNone {
		for _, sw := range SoftwareDecodeFallback(codec) {
			if c.DecoderAvailable(sw) {
				return true
			}
		}
		return false
	}
	if dec := CodecDecoderMap(codec, accel); dec != "" {
		return c.DecoderAvailable(dec)
	}
	if key := CodecHWAccelDecodeKey(codec, accel); key != "" {
		return c.DecoderAvailable(key)
	}
	return false
}

// EncodeDecodeSummary returns encode/decode availability for an encoder report row.
func (c *Capabilities) EncodeDecodeSummary(encoderName string) (encodeAvail, decodeAvail bool, decodeLabel string, decodeErr string) {
	if enc, ok := c.Encoders[encoderName]; ok {
		encodeAvail = enc.Available
	}
	binding, ok := DecodeBindingForEncoder(encoderName)
	if !ok {
		return encodeAvail, false, "", ""
	}
	decodeLabel = binding.Label
	if dec, ok := c.Decoders[binding.Key]; ok {
		decodeAvail = dec.Available
		decodeErr = dec.TestError
	}
	return encodeAvail, decodeAvail, decodeLabel, decodeErr
}

// EncoderAvailable reports whether an encoder passed detection.
func (c *Capabilities) EncoderAvailable(name string) bool {
	if enc, ok := c.Encoders[name]; ok {
		return enc.Available
	}
	return false
}

// FilterAvailable reports whether a filter is available.
func (c *Capabilities) FilterAvailable(name string) bool {
	return c.Filters[name]
}

// ProtocolAvailable reports whether a protocol is available.
func (c *Capabilities) ProtocolAvailable(name string) bool {
	return c.Protocols[name]
}
