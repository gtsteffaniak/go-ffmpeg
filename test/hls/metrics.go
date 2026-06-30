package main

import (
	"bufio"
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type resourceSample struct {
	WallMs     int64    `json:"wallMs"`
	CPUPercent float64  `json:"cpuPercent"`
	GPUPercent *float64 `json:"gpuPercent,omitempty"`
}

type resourceStats struct {
	CPUTimeSec      float64          `json:"cpuTimeSec"`
	CPUPercentAvg   float64          `json:"cpuPercentAvg"`
	CPUPercentMax   float64          `json:"cpuPercentMax"`
	GPUPercentAvg   *float64         `json:"gpuPercentAvg,omitempty"`
	GPUPercentMax   *float64         `json:"gpuPercentMax,omitempty"`
	GPUMonitor      string           `json:"gpuMonitor,omitempty"`
	Samples         []resourceSample `json:"samples,omitempty"`
}

type gpuBackend struct {
	name string
	read func() *float64
}

var gpuBackendOnce sync.Once
var gpuBackendInst gpuBackend

func getGPUBackend() gpuBackend {
	gpuBackendOnce.Do(func() {
		gpuBackendInst = detectGPUBackend()
	})
	return gpuBackendInst
}

func detectGPUBackend() gpuBackend {
	if isIntelGPU() {
		if isIntelXEDriver() {
			if paths := intelXEGTIdlePaths(); len(paths) > 0 {
				return gpuBackend{name: "xe_gtidle", read: nil}
			}
			return gpuBackend{name: "intel_xe_no_sysfs", read: nil}
		}
		if _, err := exec.LookPath("intel_gpu_top"); err == nil {
			return gpuBackend{name: "intel_gpu_top", read: readIntelGPUUtil}
		}
		return gpuBackend{name: "intel_sysfs_unavailable", read: nil}
	}
	if _, err := exec.LookPath("nvidia-smi"); err == nil {
		return gpuBackend{name: "nvidia-smi", read: readNVIDIAUtil}
	}
	return gpuBackend{name: "none", read: nil}
}

func isIntelGPU() bool {
	vendor, err := os.ReadFile("/sys/class/drm/card0/device/vendor")
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(vendor)) == "0x8086"
}

func isIntelXEDriver() bool {
	link, err := os.Readlink("/sys/class/drm/card0/device/driver")
	if err != nil {
		return false
	}
	return strings.Contains(link, "xe")
}

// intelXEGTIdlePaths returns gtidle residency counters for Intel xe (Lunar Lake+).
func intelXEGTIdlePaths() []string {
	const tileRoot = "/sys/class/drm/card0/device/tile0"
	entries, err := os.ReadDir(tileRoot)
	if err != nil {
		return nil
	}
	var paths []string
	for _, ent := range entries {
		if !strings.HasPrefix(ent.Name(), "gt") {
			continue
		}
		p := filepath.Join(tileRoot, ent.Name(), "gtidle", "idle_residency_ms")
		if _, err := os.Stat(p); err == nil {
			paths = append(paths, p)
		}
	}
	return paths
}

type intelXEReader struct {
	paths    []string
	mu       sync.Mutex
	lastIdle []uint64
	lastWall time.Time
	hasLast  bool
}

func newIntelXEReader(paths []string) *intelXEReader {
	return &intelXEReader{paths: append([]string(nil), paths...)}
}

func (r *intelXEReader) readUtil() *float64 {
	idle, err := r.readIdleValues()
	if err != nil {
		return nil
	}
	now := time.Now()

	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.hasLast {
		r.lastIdle = idle
		r.lastWall = now
		r.hasLast = true
		return nil
	}

	deltaWallMs := now.Sub(r.lastWall).Milliseconds()
	if deltaWallMs <= 0 {
		return nil
	}

	var maxUtil float64
	for i := range idle {
		if idle[i] < r.lastIdle[i] {
			continue
		}
		deltaIdle := idle[i] - r.lastIdle[i]
		if deltaIdle > uint64(deltaWallMs) {
			deltaIdle = uint64(deltaWallMs)
		}
		util := 100.0 * (1.0 - float64(deltaIdle)/float64(deltaWallMs))
		if util < 0 {
			util = 0
		}
		if util > 100 {
			util = 100
		}
		if util > maxUtil {
			maxUtil = util
		}
	}
	r.lastIdle = idle
	r.lastWall = now
	if maxUtil <= 0 && len(idle) > 0 {
		return nil
	}
	return &maxUtil
}

func (r *intelXEReader) readIdleValues() ([]uint64, error) {
	out := make([]uint64, len(r.paths))
	for i, p := range r.paths {
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		v, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
}

func readNVIDIAUtil() *float64 {
	cmd := exec.Command("nvidia-smi", "--query-gpu=utilization.gpu", "--format=csv,noheader,nounits")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	line := strings.TrimSpace(string(out))
	if idx := strings.Index(line, "\n"); idx >= 0 {
		line = line[:idx]
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(line), 64)
	if err != nil {
		return nil
	}
	return &v
}

// readIntelGPUUtil runs a short intel_gpu_top sample (requires intel-gpu-tools).
func readIntelGPUUtil() *float64 {
	cmd := exec.Command("intel_gpu_top", "-J", "-s", "100", "-o", "-")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	return parseIntelGPUTopJSON(out)
}

func parseIntelGPUTopJSON(data []byte) *float64 {
	// intel_gpu_top -J emits lines like: "period": { ... "Render/3D": 12.34 ... }
	s := string(data)
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if !strings.Contains(strings.ToLower(line), "render") && !strings.Contains(strings.ToLower(line), "video") {
			continue
		}
		if idx := strings.Index(line, ":"); idx > 0 {
			valStr := strings.TrimSpace(strings.TrimRight(line[idx+1:], ",}"))
			if v, err := strconv.ParseFloat(valStr, 64); err == nil && v >= 0 {
				return &v
			}
		}
	}
	// Fallback: scan for "busy" percentage fields in JSON
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		line := sc.Text()
		if strings.Contains(line, `"busy"`) || strings.Contains(line, `"Render"`) {
			for _, part := range strings.FieldsFunc(line, func(r rune) bool {
				return r == ':' || r == ',' || r == ' '
			}) {
				if v, err := strconv.ParseFloat(strings.Trim(part, `"`), 64); err == nil && v > 0 && v <= 100 {
					return &v
				}
			}
		}
	}
	return nil
}

type resourceMonitor struct {
	mu          sync.Mutex
	stop        chan struct{}
	done        chan struct{}
	samples     []resourceSample
	startCPU    float64
	lastCPU     float64
	lastWall    time.Time
	startWall   time.Time
	gpuBackend  gpuBackend
}

func newResourceMonitor() *resourceMonitor {
	be := getGPUBackend()
	if be.name == "xe_gtidle" {
		paths := intelXEGTIdlePaths()
		if len(paths) > 0 {
			reader := newIntelXEReader(paths)
			be.read = reader.readUtil
		}
	}
	return &resourceMonitor{
		stop:       make(chan struct{}),
		done:       make(chan struct{}),
		gpuBackend: be,
	}
}

func readProcessCPUTimeSec() float64 {
	data, err := os.ReadFile("/proc/self/stat")
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(data))
	if len(fields) < 17 {
		return 0
	}
	utime, err1 := strconv.ParseFloat(fields[13], 64)
	stime, err2 := strconv.ParseFloat(fields[14], 64)
	if err1 != nil || err2 != nil {
		return 0
	}
	clkTck := 100.0
	return (utime + stime) / clkTck
}

func (m *resourceMonitor) Start() {
	m.startCPU = readProcessCPUTimeSec()
	m.lastCPU = m.startCPU
	m.lastWall = time.Now()
	m.startWall = m.lastWall
	go func() {
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		defer close(m.done)
		for {
			select {
			case <-m.stop:
				return
			case now := <-ticker.C:
				cpuNow := readProcessCPUTimeSec()
				wallDelta := now.Sub(m.lastWall).Seconds()
				cpuDelta := cpuNow - m.lastCPU
				var cpuPct float64
				if wallDelta > 0 {
					cpuPct = (cpuDelta / wallDelta) * 100
				}
				sample := resourceSample{
					WallMs:     now.Sub(m.startWall).Milliseconds(),
					CPUPercent: cpuPct,
				}
				if m.gpuBackend.read != nil {
					sample.GPUPercent = m.gpuBackend.read()
				}
				m.mu.Lock()
				m.samples = append(m.samples, sample)
				m.mu.Unlock()
				m.lastCPU = cpuNow
				m.lastWall = now
			}
		}
	}()
}

func (m *resourceMonitor) Stop() resourceStats {
	close(m.stop)
	<-m.done
	endCPU := readProcessCPUTimeSec()

	m.mu.Lock()
	samples := append([]resourceSample(nil), m.samples...)
	m.mu.Unlock()

	stats := resourceStats{
		CPUTimeSec:    endCPU - m.startCPU,
		Samples:       samples,
		GPUMonitor:    m.gpuBackend.name,
	}
	if len(samples) == 0 {
		return stats
	}
	var cpuSum float64
	var cpuMax float64
	var gpuSum float64
	var gpuCount int
	var gpuMax float64
	for _, s := range samples {
		cpuSum += s.CPUPercent
		if s.CPUPercent > cpuMax {
			cpuMax = s.CPUPercent
		}
		if s.GPUPercent != nil {
			gpuSum += *s.GPUPercent
			gpuCount++
			if *s.GPUPercent > gpuMax {
				gpuMax = *s.GPUPercent
			}
		}
	}
	stats.CPUPercentAvg = cpuSum / float64(len(samples))
	stats.CPUPercentMax = cpuMax
	if gpuCount > 0 {
		avg := gpuSum / float64(gpuCount)
		max := gpuMax
		stats.GPUPercentAvg = &avg
		stats.GPUPercentMax = &max
	}
	return stats
}

func parseFFmpegError(stderr []byte) string {
	sc := bufio.NewScanner(bytes.NewReader(stderr))
	var lines []string
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if strings.Contains(line, "Error") || strings.Contains(line, "error") || strings.Contains(line, "Invalid") {
			lines = append(lines, line)
		}
	}
	if len(lines) == 0 && len(stderr) > 0 {
		s := string(stderr)
		if len(s) > 500 {
			s = s[len(s)-500:]
		}
		return strings.TrimSpace(s)
	}
	return strings.Join(lines, "; ")
}
