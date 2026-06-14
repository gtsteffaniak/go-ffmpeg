package ops

import (
	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
)

// RequirementSet describes what an operation needs from the capability matrix.
type RequirementSet struct {
	Encoders    []string
	Filters     []string
	Protocols   []string
	MinProfile  capabilities.BuildProfile
	NeedsEncode bool
}

// Operation describes a library operation and its requirements.
type Operation interface {
	Name() string
	Requirements() RequirementSet
}

// Supported checks whether cap satisfies the operation requirements.
func Supported(op Operation, cap *capabilities.Capabilities) (bool, []string) {
	if cap == nil {
		return false, []string{"capabilities not detected"}
	}
	req := op.Requirements()
	var reasons []string

	if req.NeedsEncode && cap.BuildProfile == capabilities.BuildDecodeOnly {
		reasons = append(reasons, "decode-only build lacks encoders")
	}
	if req.MinProfile == capabilities.BuildFull && cap.BuildProfile != capabilities.BuildFull {
		reasons = append(reasons, "requires full encode build")
	}
	for _, enc := range req.Encoders {
		if !cap.EncoderAvailable(enc) {
			reasons = append(reasons, "missing encoder "+enc)
		}
	}
	for _, f := range req.Filters {
		if !cap.FilterAvailable(f) {
			reasons = append(reasons, "missing filter "+f)
		}
	}
	for _, p := range req.Protocols {
		if !cap.ProtocolAvailable(p) {
			reasons = append(reasons, "missing protocol "+p)
		}
	}
	return len(reasons) == 0, reasons
}

var registry []Operation

// Register adds an operation to the global registry.
func Register(op Operation) {
	registry = append(registry, op)
}

// All returns all registered operations.
func All() []Operation {
	out := make([]Operation, len(registry))
	copy(out, registry)
	return out
}

// EvaluateOps updates EnabledOps and DisabledOps on cap.
func EvaluateOps(cap *capabilities.Capabilities) {
	cap.EnabledOps = nil
	cap.DisabledOps = make(map[string][]string)
	for _, op := range registry {
		ok, reasons := Supported(op, cap)
		if ok {
			cap.EnabledOps = append(cap.EnabledOps, op.Name())
		} else {
			cap.DisabledOps[op.Name()] = reasons
		}
	}
}
