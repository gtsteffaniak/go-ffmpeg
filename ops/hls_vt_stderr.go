package ops

import (
	"bytes"
	"io"
	"sync"
)

const vtDecodeFailureThreshold = 8

var (
	matroskaEBMLSeekNoise = []byte("invalid as first byte of an EBML number")
	vtDecoderCBNoise      = []byte("vt decoder cb")
	vtHardwareFailNoise   = []byte("hardware accelerator failed to decode picture")
)

type vtDecodeStderrMonitor struct {
	dst io.Writer
	mu  sync.Mutex
	// failures counts VT decode error lines in stderr.
	failures int
	// unreliable is set after sustained VT decode failures (damaged H.264 bitstream).
	unreliable bool
	pending    []byte
}

func newVTDecodeStderrMonitor(dst io.Writer) *vtDecodeStderrMonitor {
	return &vtDecodeStderrMonitor{dst: dst}
}

func (m *vtDecodeStderrMonitor) Write(p []byte) (int, error) {
	m.countVTDecodeLines(p)
	if m.dst == nil {
		return len(p), nil
	}
	filtered := filterMatroskaSeekNoise(p)
	if len(filtered) == 0 {
		return len(p), nil
	}
	_, err := m.dst.Write(filtered)
	return len(p), err
}

func (m *vtDecodeStderrMonitor) countVTDecodeLines(p []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	data := append(append([]byte(nil), m.pending...), p...)
	m.pending = nil
	for len(data) > 0 {
		i := bytes.IndexByte(data, '\n')
		if i < 0 {
			m.pending = data
			return
		}
		line := data[:i]
		data = data[i+1:]
		if bytes.Contains(line, vtDecoderCBNoise) || bytes.Contains(line, vtHardwareFailNoise) {
			m.failures++
			if m.failures >= vtDecodeFailureThreshold {
				m.unreliable = true
			}
		}
	}
}

func (m *vtDecodeStderrMonitor) VTDecodeUnreliable() bool {
	if m == nil {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.unreliable
}

// filterMatroskaSeekNoise drops benign matroska demuxer warnings emitted while
// resolving input -ss seeks on large MKV files (void/cue index entries).
func filterMatroskaSeekNoise(p []byte) []byte {
	if len(p) == 0 || !bytes.Contains(p, matroskaEBMLSeekNoise) {
		return p
	}
	lines := bytes.Split(p, []byte("\n"))
	var out bytes.Buffer
	for i, line := range lines {
		if len(line) == 0 && i == len(lines)-1 {
			continue
		}
		if bytes.Contains(line, matroskaEBMLSeekNoise) {
			continue
		}
		if out.Len() > 0 {
			out.WriteByte('\n')
		}
		out.Write(line)
	}
	if len(p) > 0 && p[len(p)-1] == '\n' && out.Len() > 0 {
		out.WriteByte('\n')
	}
	return out.Bytes()
}
