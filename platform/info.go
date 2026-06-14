package platform

// Info records host driver/device gate results.
type Info struct {
	OS                 string            `json:"os"`
	Arch               string            `json:"arch"`
	NVIDIA             bool              `json:"nvidia"`
	AMD                bool              `json:"amd"`
	Intel              bool              `json:"intel"`
	DRI                bool              `json:"dri"`
	QSV                bool              `json:"qsv"`
	QSVRuntime         bool              `json:"qsvRuntime"`
	D3D12              bool              `json:"d3d12"`
	VAAPI              bool              `json:"vaapi"`
	WSL                bool              `json:"wsl"`
	WSLGPUPartitioning bool              `json:"wslGpuPartitioning"`
	Details            map[string]string `json:"details,omitempty"`
}
