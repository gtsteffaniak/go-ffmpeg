package capabilities

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestProbeFFmpegBinaryStaticELF(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ffmpeg-static")
	writeELF64NoInterp(path)
	info := probeFFmpegBinary(path, linkProbeDeps{
		ReadFile: os.ReadFile,
		Ldd: func(string) ([]byte, error) {
			return []byte("not a dynamic executable"), nil
		},
	})
	if info.Kind != FFmpegLinkStatic {
		t.Fatalf("kind=%s want static", info.Kind)
	}
	if !gpuAccelBlockedByStaticBinary(info, "h264_vaapi") {
		t.Fatal("vaapi should be blocked on static linux")
	}
	if gpuAccelBlockedByStaticBinary(info, "v4l2m2m") {
		t.Fatal("v4l2m2m should work on static linux")
	}
}

func TestProbeFFmpegBinaryDynamicELF(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ffmpeg-dyn")
	writeELF64WithInterp(path, "/lib64/ld-linux-x86-64.so.2")
	info := probeFFmpegBinary(path, linkProbeDeps{
		ReadFile: os.ReadFile,
		Ldd: func(string) ([]byte, error) {
			return []byte("linux-vdso.so.1"), nil
		},
	})
	if info.Kind != FFmpegLinkDynamic {
		t.Fatalf("kind=%s want dynamic", info.Kind)
	}
	if info.Interp == "" {
		t.Fatal("expected interpreter path")
	}
}

func writeELF64NoInterp(path string) {
	hdr := make([]byte, 64)
	hdr[0] = 0x7f
	hdr[1], hdr[2], hdr[3] = 'E', 'L', 'F'
	hdr[4] = 2 // ELFCLASS64
	hdr[5] = 1 // ELFDATA2LSB
	binary.LittleEndian.PutUint16(hdr[18:20], 1) // e_machine x86_64
	binary.LittleEndian.PutUint64(hdr[32:40], 64) // e_phoff
	binary.LittleEndian.PutUint16(hdr[54:56], 56) // e_phentsize
	binary.LittleEndian.PutUint16(hdr[56:58], 0)  // e_phnum
	_ = os.WriteFile(path, hdr, 0o755)
}

func writeELF64WithInterp(path string, interp string) {
	hdr := make([]byte, 64)
	hdr[0] = 0x7f
	hdr[1], hdr[2], hdr[3] = 'E', 'L', 'F'
	hdr[4] = 2
	hdr[5] = 1
	binary.LittleEndian.PutUint16(hdr[18:20], 1)
	binary.LittleEndian.PutUint64(hdr[32:40], 64)
	binary.LittleEndian.PutUint16(hdr[54:56], 56)
	binary.LittleEndian.PutUint16(hdr[56:58], 1)

	ph := make([]byte, 56)
	binary.LittleEndian.PutUint32(ph[0:4], 3) // PT_INTERP
	binary.LittleEndian.PutUint64(ph[8:16], 120)
	binary.LittleEndian.PutUint64(ph[32:40], uint64(len(interp)+1))

	interpBytes := append([]byte(interp), 0)
	buf := make([]byte, 120+len(interpBytes))
	copy(buf, hdr)
	copy(buf[64:], ph)
	copy(buf[120:], interpBytes)
	_ = os.WriteFile(path, buf, 0o755)
}
