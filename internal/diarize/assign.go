// Package diarize tells who speaks when in a recording, on this machine
// (sherpa-onnx: pyannote segmentation and a speaker embedding model).
package diarize

import "transflowy/internal/library"

// Turn is a stretch of one speaker's voice. Speakers are numbered from 0 in
// the order sherpa-onnx finds them.
type Turn struct {
	Start, End float64
	Speaker    int
}

// Assign gives each transcript line to the speaker whose turns cover most of
// it, for the lines match accepts. A line no turn touches keeps its speaker.
// It returns how many lines changed.
func Assign(segs []library.Segment, turns []Turn, match func(speaker int) bool) int {
	n := 0
	for i := range segs {
		s := &segs[i]
		if !match(s.Speaker) {
			continue
		}
		cover := map[int]float64{}
		for _, t := range turns {
			if o := min(s.End, t.End) - max(s.Start, t.Start); o > 0 {
				cover[t.Speaker] += o
			}
		}
		best, bestCover := s.Speaker, 0.0
		for sp, c := range cover {
			if c > bestCover || (c == bestCover && sp < best) {
				best, bestCover = sp, c
			}
		}
		if bestCover > 0 && best != s.Speaker {
			s.Speaker = best
			n++
		}
	}
	return n
}
