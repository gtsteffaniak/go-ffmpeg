// Package capabilities discovers what an ffmpeg binary can do and which hardware
// encoders are actually usable on the host.
//
// # Hardware encoder detection
//
// Detection runs once per [Capabilities] snapshot (typically at library New or
// Reload). Results are stored on [EncoderCapability] and [EncoderProbeResult]
// and are not updated when a transcode fails. The library does not re-probe,
// retry probes, or fall back to software encoders automatically; call Reload
// only when you intentionally re-run full detection after fixing drivers or
// configuration.
//
// Layered hardware probes run in order:
//
//  1. Binary link — [ProbeFFmpegBinary] classifies static vs dynamic ffmpeg on Linux.
//  2. Host prerequisites — [CheckHostPrerequisites] checks render nodes, libva, NVIDIA libs, WSL paths.
//  3. Functional probe — [ProbeHardwareEncoder] runs a short encode to null with a timeout.
//
// Each failure sets an [HWProbeCode], a human [HintForProbeCode], and optional Detail
// (truncated ffmpeg stderr). When PCI GPU presence is known but the stack is missing,
// [ApplyHostRemediation] may upgrade the code to [HWProbeGPUPresentDriverMissing] with
// distro-specific install guidance from [DetectionHost].
//
// See docs/hardware-detection.md for platform matrices and error code tables.
//
// # Concurrency
//
// [Detect] and probe helpers use the caller's context for timeouts. The returned
// [Capabilities] value is safe to read from multiple goroutines after detection
// completes; do not mutate it concurrently.
package capabilities
