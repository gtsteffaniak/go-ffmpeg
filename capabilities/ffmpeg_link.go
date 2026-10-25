package capabilities

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// FFmpegLinkKind describes how the ffmpeg binary is linked on the host.
type FFmpegLinkKind string

const (
	FFmpegLinkDynamic FFmpegLinkKind = "dynamic"
	FFmpegLinkStatic  FFmpegLinkKind = "static"
	FFmpegLinkUnknown FFmpegLinkKind = "unknown"
)

// FFmpegBinaryInfo records compile-time listing support vs runtime dlopen ability.
type FFmpegBinaryInfo struct {
	Kind       FFmpegLinkKind `json:"kind"`
	Interp     string         `json:"interp,omitempty"`
	LddSummary string         `json:"lddSummary,omitempty"`
}

// linkProbeDeps allows tests to stub filesystem and ldd output.
type linkProbeDeps struct {
	ReadFile func(string) ([]byte, error)
	Ldd      func(string) ([]byte, error)
}

func defaultLinkProbeDeps() linkProbeDeps {
	return linkProbeDeps{
		ReadFile: readFileHead,
		Ldd: func(path string) ([]byte, error) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			return exec.CommandContext(ctx, "ldd", path).CombinedOutput()
		},
	}
}

func readFileHead(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, 1<<16))
}

// ProbeFFmpegBinary inspects the ffmpeg binary for static vs dynamic linking.
// On non-Linux hosts this returns dynamic unless inspection fails.
func ProbeFFmpegBinary(ffmpegPath string) FFmpegBinaryInfo {
	return probeFFmpegBinary(ffmpegPath, defaultLinkProbeDeps())
}

func probeFFmpegBinary(ffmpegPath string, deps linkProbeDeps) FFmpegBinaryInfo {
	if ffmpegPath == "" {
		return FFmpegBinaryInfo{Kind: FFmpegLinkUnknown}
	}
	if runtime.GOOS != "linux" {
		return FFmpegBinaryInfo{Kind: FFmpegLinkDynamic}
	}
	kind, interp, isELF := elfLinkKind(ffmpegPath, deps.ReadFile)
	if !isELF {
		return FFmpegBinaryInfo{Kind: FFmpegLinkUnknown}
	}
	if kind == FFmpegLinkStatic {
		return FFmpegBinaryInfo{Kind: FFmpegLinkStatic}
	}
	if kind == FFmpegLinkDynamic {
		return FFmpegBinaryInfo{Kind: FFmpegLinkDynamic, Interp: interp}
	}
	out, err := deps.Ldd(ffmpegPath)
	summary := strings.TrimSpace(string(out))
	lower := strings.ToLower(summary)
	if strings.Contains(lower, "not a dynamic executable") || strings.Contains(lower, "statically linked") {
		return FFmpegBinaryInfo{Kind: FFmpegLinkStatic, LddSummary: truncateProbeDetail(summary, 200)}
	}
	if err != nil {
		return FFmpegBinaryInfo{Kind: FFmpegLinkUnknown, LddSummary: truncateProbeDetail(summary, 200)}
	}
	return FFmpegBinaryInfo{Kind: FFmpegLinkDynamic, LddSummary: truncateProbeDetail(summary, 200)}
}

// gpuAccelBlockedByStaticBinary reports whether a hardware backend cannot dlopen
// userspace drivers when ffmpeg is statically linked on Linux.
func gpuAccelBlockedByStaticBinary(info FFmpegBinaryInfo, encoderKind string) bool {
	if info.Kind != FFmpegLinkStatic || runtime.GOOS != "linux" {
		return false
	}
	switch encoderKind {
	case "v4l2m2m":
		return false
	case "software", "native":
		return false
	default:
		return true
	}
}

const (
	elfMagic0  = 0x7f
	elfMagic1  = 'E'
	elfMagic2  = 'L'
	elfMagic3  = 'F'
	elfClass64 = 2
	elfDataLE  = 1
	ptInterp   = 3
)

func elfLinkKind(path string, readFile func(string) ([]byte, error)) (FFmpegLinkKind, string, bool) {
	data, err := readFile(path)
	if err != nil || len(data) < 64 {
		return FFmpegLinkUnknown, "", false
	}
	if data[0] != elfMagic0 || data[1] != elfMagic1 || data[2] != elfMagic2 || data[3] != elfMagic3 {
		return FFmpegLinkUnknown, "", false
	}
	if data[4] != elfClass64 {
		// 32-bit builds are rare; treat missing PT_INTERP the same way.
		if data[4] != 1 {
			return FFmpegLinkUnknown, "", true
		}
	}
	is64 := data[4] == elfClass64
	if data[5] != elfDataLE {
		return FFmpegLinkUnknown, "", true
	}

	var ehdr struct {
		e_phoff     uint64
		e_phentsize uint16
		e_phnum     uint16
	}
	if is64 {
		if len(data) < 64 {
			return FFmpegLinkUnknown, "", true
		}
		ehdr.e_phoff = binary.LittleEndian.Uint64(data[32:40])
		ehdr.e_phentsize = binary.LittleEndian.Uint16(data[54:56])
		ehdr.e_phnum = binary.LittleEndian.Uint16(data[56:58])
	} else {
		if len(data) < 52 {
			return FFmpegLinkUnknown, "", true
		}
		ehdr.e_phoff = uint64(binary.LittleEndian.Uint32(data[28:32]))
		ehdr.e_phentsize = binary.LittleEndian.Uint16(data[42:44])
		ehdr.e_phnum = binary.LittleEndian.Uint16(data[44:46])
	}

	off := int(ehdr.e_phoff)
	ent := int(ehdr.e_phentsize)
	n := int(ehdr.e_phnum)
	if n == 0 {
		return FFmpegLinkStatic, "", true
	}
	if ent == 0 || off < 0 || off+ent*n > len(data) {
		return FFmpegLinkUnknown, "", true
	}

	var interp string
	for i := 0; i < n; i++ {
		ph := data[off+i*ent : off+(i+1)*ent]
		var pType uint32
		var pOffset, pFilesz uint64
		if is64 {
			if len(ph) < 56 {
				continue
			}
			pType = binary.LittleEndian.Uint32(ph[0:4])
			pOffset = binary.LittleEndian.Uint64(ph[8:16])
			pFilesz = binary.LittleEndian.Uint64(ph[32:40])
		} else {
			if len(ph) < 32 {
				continue
			}
			pType = binary.LittleEndian.Uint32(ph[0:4])
			pOffset = uint64(binary.LittleEndian.Uint32(ph[4:8]))
			pFilesz = uint64(binary.LittleEndian.Uint32(ph[16:20]))
		}
		if pType != ptInterp {
			continue
		}
		start := int(pOffset)
		end := start + int(pFilesz)
		if start < 0 || end > len(data) {
			continue
		}
		interp = string(bytes.TrimRight(data[start:end], "\x00"))
		break
	}
	if interp == "" {
		return FFmpegLinkStatic, "", true
	}
	return FFmpegLinkDynamic, interp, true
}
