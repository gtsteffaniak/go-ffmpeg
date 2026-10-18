package platform

// GPUMode classifies a resolved GPU config value.
type GPUMode string

const (
	GPUModeAuto     GPUMode = "auto"
	GPUModeIGPU     GPUMode = "igpu"
	GPUModeDGPU     GPUMode = "dgpu"
	GPUModeDevice   GPUMode = "device"
	GPUModeSoftware GPUMode = "software"
)

// GPUChoice describes a resolved GPU selection from user config.
type GPUChoice struct {
	Enabled        bool    `json:"enabled"`
	SoftwareOnly   bool    `json:"softwareOnly,omitempty"`
	AutoSelect     bool    `json:"autoSelect,omitempty"`
	Mode           GPUMode `json:"mode,omitempty"`
	RequestedSpec  string  `json:"requestedSpec,omitempty"` // deprecated: use Label
	Label          string  `json:"label,omitempty"`
	DeviceID       string  `json:"deviceId,omitempty"`
	RenderDevice   string  `json:"renderDevice,omitempty"`
	Vendor         string  `json:"vendor,omitempty"`
	Name           string  `json:"name,omitempty"`
	DXGIIndex      int     `json:"dxgiIndex,omitempty"`
	LUID           string  `json:"luid,omitempty"`
	CUDAIndex      int     `json:"cudaIndex,omitempty"`
	Integrated     bool    `json:"integrated,omitempty"`
	Fallback       bool    `json:"fallback,omitempty"`
	FallbackReason string  `json:"fallbackReason,omitempty"`
	Warning        string  `json:"warning,omitempty"`
}
