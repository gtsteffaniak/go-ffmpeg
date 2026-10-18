package platform

// PinningConfidence describes how reliably a backend is pinned to a GPU device.
type PinningConfidence string

const (
	PinningExact      PinningConfidence = "exact"
	PinningBestEffort PinningConfidence = "best-effort"
	PinningUnknown    PinningConfidence = "unknown"
)

// BackendPinning records per-backend pinning confidence for the selected device.
type BackendPinning struct {
	NVENC PinningConfidence `json:"nvenc,omitempty"`
	AMF   PinningConfidence `json:"amf,omitempty"`
	QSV   PinningConfidence `json:"qsv,omitempty"`
	D3D11 PinningConfidence `json:"d3d11va,omitempty"`
	VAAPI PinningConfidence `json:"vaapi,omitempty"`
	CUDA  PinningConfidence `json:"cuda,omitempty"`
	Note  string            `json:"note,omitempty"`
}
