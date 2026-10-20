package diarize

import (
	"testing"

	"transflowy/internal/library"
)

func TestAssign(t *testing.T) {
	segs := []library.Segment{
		{Start: 0, End: 4, Speaker: library.UnknownSpeaker},  // 3 s of speaker 1, 1 s of speaker 0
		{Start: 5, End: 8, Speaker: library.UnknownSpeaker},  // no turn
		{Start: 9, End: 12, Speaker: library.SpeakerMe},      // the microphone: not touched
		{Start: 13, End: 15, Speaker: library.SpeakerOthers}, // the system track's lines
	}
	turns := []Turn{{0, 1, 0}, {1, 4, 1}, {13, 15, 2}, {9, 12, 1}}
	n := Assign(segs, turns, func(s int) bool { return s != library.SpeakerMe })
	if n != 2 || segs[0].Speaker != 1 || segs[1].Speaker != library.UnknownSpeaker || segs[2].Speaker != library.SpeakerMe || segs[3].Speaker != 2 {
		t.Fatalf("n=%d %+v", n, segs)
	}
}

func TestTidyMergesFragmentsAndRanksBySpeech(t *testing.T) {
	turns := []Turn{
		{0, 100, 7},   // speaks most
		{100, 160, 3}, // second
		{160, 163, 9}, // fragment next to speaker 3
		{163, 170, 4}, // fragment next to speaker 3
		{170, 230, 7},
	}
	got := Tidy(turns)
	want := []int{0, 1, 1, 0, 0}
	for i, g := range got {
		if g.Speaker != want[i] {
			t.Fatalf("turn %d: speaker %d, want %d (%+v)", i, g.Speaker, want[i], got)
		}
	}
	short := []Turn{{0, 5, 3}, {5, 9, 8}}
	if got := Tidy(short); got[0].Speaker != 3 || got[1].Speaker != 8 {
		t.Fatalf("a short recording is left alone: %+v", got)
	}
}
