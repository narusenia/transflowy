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
