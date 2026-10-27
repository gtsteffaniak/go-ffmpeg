package encode

import "strconv"

// DefaultFrameQuality is the JPEG/WebP quality used when FrameOptions.Quality is 0.
const DefaultFrameQuality = 85

// NormalizeFrameQuality clamps quality to 1–100 (higher is better) and maps 0 to DefaultFrameQuality.
func NormalizeFrameQuality(quality int) int {
	if quality == 0 {
		return DefaultFrameQuality
	}
	if quality < 1 {
		return 1
	}
	if quality > 100 {
		return 100
	}
	return quality
}

// MJPEGQualityQScale maps frame quality 1–100 to ffmpeg mjpeg -q:v (2–31, lower is better).
func MJPEGQualityQScale(quality int) string {
	q := NormalizeFrameQuality(quality)
	// round(31 - q*29/100), clamped 2..31
	scale := 31 - (q*29+50)/100
	if scale < 2 {
		scale = 2
	}
	if scale > 31 {
		scale = 31
	}
	return strconv.Itoa(scale)
}

// WebPQuality returns libwebp -quality for frame quality 1–100.
func WebPQuality(quality int) int {
	return NormalizeFrameQuality(quality)
}
