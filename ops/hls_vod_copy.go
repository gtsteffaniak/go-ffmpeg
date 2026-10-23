package ops

import (
	"fmt"
	"math"
	"strings"
)

const (
	// DefaultHLSVODForwardGapSec is the Jellyfin-style forward wait window before restarting a job.
	DefaultHLSVODForwardGapSec = 24.0
	// vodRemuxInputSeekPadSec nudges remux input -ss past the target keyframe (Jellyfin DynamicHls).
	vodRemuxInputSeekPadSec = 0.5
)

// VODCopySegmentCutTimesRel returns ffmpeg -segment_times for a copy/remux restart job,
// rebased to keyframeSeekSec (the media time of the input -ss keyframe).
// Cuts are at each planned segment boundary after startIndex.
func VODCopySegmentCutTimesRel(list VODSegmentList, keyframeSeekSec float64, startIndex int) []float64 {
	if startIndex < 0 || startIndex >= list.Count() {
		return nil
	}
	var times []float64
	for j := startIndex; j < list.Count()-1; j++ {
		t := list.Starts[j+1] - keyframeSeekSec
		if t > 0.0005 {
			times = append(times, t)
		}
	}
	return times
}

func vodCopyUsesExplicitSegmentCuts(opts HLSVODJobOptions) bool {
	if !opts.VideoCopyPipeline() || opts.SegmentList == nil {
		return false
	}
	return opts.SegmentList.Count() > 0
}

func vodCopyKeyframeSeekSec(opts HLSVODJobOptions) float64 {
	mediaSeek := opts.StartSec
	if len(opts.KeyframeSeekTimes) > 0 {
		mediaSeek = KeyframeSeekBefore(opts.KeyframeSeekTimes, opts.StartSec)
	}
	if mediaSeek < 0 {
		mediaSeek = 0
	}
	return mediaSeek
}

// appendVODCopyExplicitSegmentMuxerArgs appends -f segment argv that cuts on the absolute
// keyframe grid (via -segment_times) so mid-file copy restarts match a from-zero job.
func appendVODCopyExplicitSegmentMuxerArgs(args []string, opts HLSVODJobOptions) []string {
	list := *opts.SegmentList
	kfSeek := vodCopyKeyframeSeekSec(opts)
	cutTimes := VODCopySegmentCutTimesRel(list, kfSeek, opts.StartIndex)
	timesStr := strings.Trim(strings.Join(floatsToArgStrings(cutTimes), ","), ",")

	args = append(args,
		"-f", "segment",
		"-segment_format", "mp4",
		"-segment_header_filename", HLSVODInitFilename,
		"-segment_format_options", "movflags=frag_keyframe+empty_moov+default_base_moof+skip_sidx+frag_discont",
		"-segment_start_number", fmt.Sprintf("%d", opts.StartIndex),
		"-segment_list", HLSVODInternalSegmentListName,
		"-segment_list_type", "csv",
		"-segment_list_flags", "+live",
	)
	if timesStr != "" {
		args = append(args, "-segment_times", timesStr)
	}
	args = append(args, fmt.Sprintf("%s/%s", HLSVODSegmentSubdir, "%05d.m4s"))
	return args
}

func floatsToArgStrings(vals []float64) []string {
	out := make([]string, len(vals))
	for i, v := range vals {
		// Floor to milliseconds with a 1ms bias so the emitted cut lands just
		// before its planned boundary: rounding up could put it past the
		// keyframe it names and the muxer would cut at the next keyframe.
		out[i] = fmt.Sprintf("%.3f", math.Floor((v-0.001)*1000)/1000)
	}
	return out
}
