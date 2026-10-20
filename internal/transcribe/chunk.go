// Package transcribe cuts audio into chunks and sends them to a speech engine.
package transcribe

import (
	"math"
	"sort"
)

const (
	SampleRate = 16000
	frameLen   = SampleRate / 50 // 20 ms
)

// Params say how long chunks may be, in seconds.
type Params struct {
	Target     float64 // look for a silence to cut at once a chunk is this long
	Max        float64 // cut here even in the middle of speech
	MinSilence float64 // a pause this long is a place to cut
}

// Whisper decides one language per ~30 s window, so local chunks stay short
// and each one is detected on its own. The cloud engine keeps one speaker
// numbering per request, so its chunks are long (the request limit is about
// 25 MB of WAV, 13 minutes).
var (
	LocalParams = Params{Target: 8, Max: 15, MinSilence: 0.3}
	CloudParams = Params{Target: 300, Max: 600, MinSilence: 0.5}
)

// Span is a chunk: samples [Start, End). Speech is false for a chunk that is
// only silence or noise, which is not sent (engines invent text for silence).
type Span struct {
	Start, End int
	Speech     bool
}

// Plan cuts pcm[from:] into chunks at pauses. With final false it stops before
// a tail that is too short to cut yet and returns where to continue; with
// final true the tail becomes the last chunk.
//
// ponytail: an energy threshold, not a trained VAD. Steady loud noise can hide
// the pauses (then chunks are cut at Max); swap in silero (sherpa-onnx) with
// speaker separation if that matters.
func Plan(pcm []int16, from int, final bool, p Params) (spans []Span, next int) {
	n := (len(pcm) - from) / frameLen
	if n <= 0 {
		return nil, from
	}
	rms := make([]float64, n)
	for i := range rms {
		var sum float64
		for _, v := range pcm[from+i*frameLen : from+(i+1)*frameLen] {
			sum += float64(v) * float64(v)
		}
		rms[i] = math.Sqrt(sum / frameLen)
	}
	sorted := append([]float64(nil), rms...)
	sort.Float64s(sorted)
	// The threshold sits above the room's noise (3x the quietest tenth) but
	// below speech (a sixth of the loud end), so continuous speech with
	// little silence is not mistaken for silence. Nothing above -55 dBFS at
	// all is silence.
	loudEnd := sorted[len(sorted)*95/100]
	thr := math.Min(math.Max(3*sorted[len(sorted)/10], 100), loudEnd/6)
	if loudEnd < 60 {
		thr = math.Inf(1)
	}

	target, maxF := int(p.Target*50), int(p.Max*50)
	minSil := max(int(p.MinSilence*50), 1)
	start := 0
	emit := func(end int) {
		loud := 0
		for _, r := range rms[start:end] {
			if r >= thr {
				loud++
			}
		}
		spans = append(spans, Span{from + start*frameLen, from + end*frameLen, loud >= 5})
		start = end
	}
	for start < n {
		cut := -1
		for i := start + target; i < min(start+maxF, n); i++ {
			j := i
			for j < n && rms[j] < thr {
				j++
			}
			if j-i >= minSil && j < n { // a pause that ends: the speech after it is real
				cut = i + (j-i)/2
				break
			}
			i = max(i, j) // skip past a run too short to cut at
		}
		switch {
		case cut >= 0:
			emit(cut)
		case n-start >= maxF:
			emit(start + maxF)
		case final:
			emit(n)
		default:
			return spans, from + start*frameLen
		}
	}
	return spans, from + start*frameLen
}
