package library

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newRec(t *testing.T) (Store, string) {
	st := Store{Root: t.TempDir()}
	m, _ := st.Create("x", time.Now())
	st.SaveTranscript(m.ID, Transcript{Segments: []Segment{
		{Start: 1, Text: "テスト環境", Original: "テスト環境"},
		{Start: 5, Text: "テスト", Original: "テスト"},
	}})
	return st, m.ID
}

func TestEditAndReset(t *testing.T) {
	st, id := newRec(t)
	if err := st.EditSegment(id, 1, 5, " 本番 "); err != nil {
		t.Fatal(err)
	}
	r, _ := st.Get(id)
	if s := r.Transcript.Segments[1]; s.Text != "本番" || s.Original != "テスト" {
		t.Fatalf("%+v", s)
	}
	if err := st.EditSegment(id, 1, 99, "x"); err != ErrStaleSegment {
		t.Fatalf("wrong start: %v", err)
	}
	if err := st.EditSegment(id, 7, 5, "x"); err != ErrStaleSegment {
		t.Fatalf("bad index: %v", err)
	}
	st.ResetSegment(id, 1, 5)
	if r, _ = st.Get(id); r.Transcript.Segments[1].Text != "テスト" {
		t.Fatalf("reset: %+v", r.Transcript.Segments[1])
	}
}

func TestReplaceAndDictionary(t *testing.T) {
	st, id := newRec(t)
	if n, err := st.Replace(id, "テスト", "検証"); n != 2 || err != nil {
		t.Fatalf("%d %v", n, err)
	}
	if n, _ := st.Replace(id, "", "x"); n != 0 {
		t.Fatal("empty find replaced")
	}
	st.AddToDictionary("検証")
	st.AddToDictionary("検証")
	st.AddToDictionary("Kubernetes")
	b, _ := os.ReadFile(filepath.Join(st.Root, "dictionary.txt"))
	if string(b) != "検証\nKubernetes\n" {
		t.Fatalf("%q", b)
	}
}

func TestStaleSummary(t *testing.T) {
	st, id := newRec(t)
	st.SaveSummary(id, "議事録", "古い")
	d, _ := st.Path(id)
	old := time.Now().Add(-time.Hour)
	os.Chtimes(filepath.Join(d, "summary-議事録.md"), old, old)
	r, _ := st.Get(id)
	if len(r.Stale) != 1 || r.Stale[0] != "議事録" {
		t.Fatalf("stale: %v", r.Stale)
	}
	st.SaveSummary(id, "議事録", "新しい")
	if r, _ = st.Get(id); len(r.Stale) != 0 {
		t.Fatalf("fresh summary is stale: %v", r.Stale)
	}
}

func TestSetSpeakers(t *testing.T) {
	st, id := newRec(t)
	st.SetSpeakers(id, map[string]string{"0": " 田中 ", "1": ""})
	r, _ := st.Get(id)
	if len(r.Transcript.Speakers) != 1 || r.Transcript.Speakers["0"] != "田中" {
		t.Fatalf("%v", r.Transcript.Speakers)
	}
	st.SetSpeakers(id, map[string]string{})
	if r, _ = st.Get(id); r.Transcript.Speakers != nil {
		t.Fatalf("%v", r.Transcript.Speakers)
	}
}

func TestAppendSegmentsKeepsTimeOrder(t *testing.T) {
	st, id := newRec(t) // lines at 1 and 5
	st.AppendSegments(id, []Segment{{Start: 3, Text: "x"}, {Start: 0.5, Text: "y"}})
	r, _ := st.Get(id)
	var starts []float64
	for _, s := range r.Transcript.Segments {
		starts = append(starts, s.Start)
	}
	if len(starts) != 4 || starts[0] != 0.5 || starts[1] != 1 || starts[2] != 3 || starts[3] != 5 {
		t.Fatalf("%v", starts)
	}
}

func TestSummaryNamesAreRestrictedAndRemovable(t *testing.T) {
	st, id := newRec(t)
	if err := st.SaveSummary(id, "../x", "a"); err == nil {
		t.Fatal("a path as a name must be refused")
	}
	if err := st.RemoveSummary(id, "../x"); err == nil {
		t.Fatal("a path as a name must be refused")
	}
	st.SaveSummary(id, "メモ", "手で書いた")
	if r, _ := st.Get(id); len(r.Summaries) != 1 {
		t.Fatalf("%v", r.Summaries)
	}
	if err := st.RemoveSummary(id, "メモ"); err != nil {
		t.Fatal(err)
	}
	if err := st.RemoveSummary(id, "メモ"); err != nil {
		t.Fatalf("removing twice: %v", err)
	}
	if r, _ := st.Get(id); len(r.Summaries) != 0 {
		t.Fatalf("%v", r.Summaries)
	}
}
