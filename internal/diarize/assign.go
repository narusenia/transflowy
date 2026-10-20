// Package diarize tells who speaks when in a recording, on this machine
// (sherpa-onnx: pyannote segmentation and a speaker embedding model).
package diarize

import (
	"sort"

	"transflowy/internal/library"
)

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

// Tidy merges the speakers who say next to nothing into the nearest turn of a
// real speaker and numbers the speakers by how much they speak (0 is the one
// who speaks most). Clustering splits one voice into many when the audio is
// noisy or the voice shifts; those fragments are not people. A short
// recording is left alone: there is too little to tell.
func Tidy(turns []Turn) []Turn {
	total := map[int]float64{}
	all := 0.0
	for _, t := range turns {
		d := t.End - t.Start
		total[t.Speaker] += d
		all += d
	}
	if all < minSpeech {
		return turns
	}
	minor := max(minSeconds, minShare*all)
	var majors []Turn
	for _, t := range turns {
		if total[t.Speaker] >= minor {
			majors = append(majors, t)
		}
	}
	if len(majors) == 0 {
		return turns
	}
	out := make([]Turn, len(turns))
	for i, t := range turns {
		out[i] = t
		if total[t.Speaker] >= minor {
			continue
		}
		best, gap := t.Speaker, 1e18
		for _, m := range majors {
			if g := max(m.Start-t.End, t.Start-m.End, 0); g < gap {
				best, gap = m.Speaker, g
			}
		}
		out[i].Speaker = best
	}
	// number by speech, most first
	left := map[int]float64{}
	for _, t := range out {
		left[t.Speaker] += t.End - t.Start
	}
	ids := make([]int, 0, len(left))
	for id := range left {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		return left[ids[i]] > left[ids[j]] || left[ids[i]] == left[ids[j]] && ids[i] < ids[j]
	})
	rank := map[int]int{}
	for n, id := range ids {
		rank[id] = n
	}
	for i := range out {
		out[i].Speaker = rank[out[i].Speaker]
	}
	return out
}

const (
	minSpeech  = 60.0 // seconds of speech below which Tidy does nothing
	minSeconds = 10.0 // a speaker with less speech than this is a fragment...
	minShare   = 0.02 // ...or with less than this share of all speech
)
