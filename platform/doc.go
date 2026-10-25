// Package platform collects host OS, GPU device, and driver context used during
// capability detection.
//
// Info comes from Detect (driver nodes, WSL, VideoToolbox, etc.). GPUPresence and
// DetectGPUPresence report PCI/vendor signals without requiring working VA or NVENC
// stacks, so layered detection can distinguish "no GPU" from "GPU present, driver missing".
//
// Detection helpers in this package are read-only snapshots: they do not cache across
// calls and are safe to invoke from the capabilities detection pass. They do not
// mutate global state.
package platform
