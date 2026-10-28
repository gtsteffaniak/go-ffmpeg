package capabilities

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// ffmpegLogPrefix matches "[component @ 0xADDR] " prefixes in ffmpeg stderr.
var ffmpegLogPrefix = regexp.MustCompile(`^\[[^\]]*@ 0x[0-9a-fA-F]+\]\s*(\[[^\]]*@ 0x[0-9a-fA-F]+\]\s*)*`)

// tagSummaryLine matches the "[hardware] ..." summary lines built by hwTroubleshoot.
var tagSummaryLine = regexp.MustCompile(`^\[(hardware|driver|runtime|permission|compile)\]\s`)

var probeDetailKeywords = []string{
	"error", "fail", "unsupported", "not supported", "no usable", "cannot", "unable",
	"permission denied", "no capable", "not found", "no such", "impossible to convert",
}

var probeDetailNoise = []string{
	"task finished with error code",
	"terminating thread",
	"could not open encoder before eof",
	"error while opening encoder - maybe incorrect parameters",
	"nothing was written into output file",
	"conversion failed",
	"error sending frames to consumers",
}

// compactProbeDetail reduces raw multi-line ffmpeg stderr to one short, readable
// line: only error-like lines, without "[x @ 0x...]" prefixes, noise lines or
// duplicates, cut at a word boundary. It never contains newlines.
func compactProbeDetail(raw string, max int) string {
	seen := map[string]struct{}{}
	var picked, summaries []string
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(ffmpegLogPrefix.ReplaceAllString(strings.TrimSpace(line), ""))
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		if containsAny(lower, probeDetailNoise) || !containsAny(lower, probeDetailKeywords) {
			continue
		}
		if _, dup := seen[lower]; dup {
			continue
		}
		seen[lower] = struct{}{}
		// "[hardware] ..." style summaries are produced by hwTroubleshoot and
		// duplicate the probe hint; only keep them when nothing more specific exists.
		if tagSummaryLine.MatchString(line) {
			summaries = append(summaries, line)
			continue
		}
		picked = append(picked, line)
	}
	if len(picked) == 0 {
		picked = summaries
	}
	if len(picked) == 0 {
		for _, line := range strings.Split(raw, "\n") {
			line = strings.TrimSpace(ffmpegLogPrefix.ReplaceAllString(strings.TrimSpace(line), ""))
			if line != "" {
				picked = append(picked, line)
				break
			}
		}
	}
	return cutAtWord(strings.Join(picked, "; "), max)
}

func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// cutAtWord truncates s to at most max bytes on a word/rune boundary, adding an ellipsis.
func cutAtWord(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	cut := s[:max]
	if i := strings.LastIndexAny(cut, " ;,"); i > max/2 {
		cut = cut[:i]
	}
	for !utf8.ValidString(cut) && len(cut) > 0 {
		cut = cut[:len(cut)-1]
	}
	return strings.TrimRight(cut, " ;,") + "…"
}
