package transcribe

import (
	"math"
	"testing"
)

func tone(sec, amp float64) []int16 {
	out := make([]int16, int(sec*SampleRate))
	for i := range out {
		out[i] = int16(amp * math.Sin(2*math.Pi*300*float64(i)/SampleRate))
	}
	return out
}

func TestPlan(t *testing.T) {
	var pcm []int16
	for _, part := range [][]int16{tone(5, 8000), tone(1, 0), tone(6, 8000), tone(0.6, 0), tone(20.6, 8000)} {
		pcm = append(pcm, part...)
	}
	sec := func(i int) float64 { return float64(i) / SampleRate }

	short := Params{Target: 8, Max: 15, MinSilence: 0.3} // makes the forced cut happen
	spans, next := Plan(pcm, 0, true, short)
	if len(spans) != 3 {
		t.Fatalf("want 3 chunks, got %+v", spans)
	}
	if c := sec(spans[0].End); c < 12 || c > 12.7 { // cut inside the pause at 12.0-12.6
		t.Errorf("first cut at %.2f s, want inside the pause", c)
	}
	if c := sec(spans[1].End - spans[1].Start); c > 15 || c < 10 { // continuous speech: forced near Max
		t.Errorf("second chunk %.2f s, want 10..15", c)
	}
	if next != len(pcm) || spans[2].End != len(pcm) {
		t.Errorf("final must consume everything: next=%d", next)
	}

	// Not final: the tail waits for more audio and is returned as the resume point.
	spans, next = Plan(pcm, 0, false, short)
	if len(spans) != 2 || next != spans[1].End {
		t.Errorf("non-final: %+v next=%d", spans, next)
	}

	// With no pause at all the cut goes to the quietest place before Max.
	dip := append(append(tone(12, 8000), tone(0.2, 1500)...), tone(20, 8000)...) // a soft spot at 12.0-12.2 s
	if sp, _ := Plan(dip, 0, true, Params{Target: 8, Max: 15, MinSilence: 0.3}); len(sp) < 2 || sec(sp[0].End) < 11.9 || sec(sp[0].End) > 12.3 {
		t.Errorf("cut not at the dip: %+v", sp)
	}

	// Silence alone is a chunk without speech.
	if s, _ := Plan(make([]int16, 10*SampleRate), 0, true, LocalParams); len(s) != 1 || s[0].Speech {
		t.Errorf("silence: %+v", s)
	}
}
