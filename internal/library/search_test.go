package library

import (
	"testing"
	"time"
)

func TestSearch(t *testing.T) {
	st := Store{Root: t.TempDir()}
	a, _ := st.Create("週次定例", time.Now().Add(-time.Hour))
	b, _ := st.Create("商談", time.Now())
	st.SaveTranscript(a.ID, Transcript{Segments: []Segment{{Start: 12, Text: "Budget は来月"}, {Start: 30, Text: "別の話"}}})
	st.SaveSummary(b.ID, "議事録", "## 概要\n予算が決まった\n")

	hits, err := st.Search("budget", 10)
	if err != nil || len(hits) != 1 || hits[0].Kind != "transcript" || hits[0].Time != 12 || hits[0].ID != a.ID {
		t.Fatalf("transcript: %+v %v", hits, err)
	}
	hits, _ = st.Search("予算", 10)
	if len(hits) != 1 || hits[0].Kind != "summary" || hits[0].Template != "議事録" || hits[0].Snippet != "予算が決まった" {
		t.Fatalf("summary: %+v", hits)
	}
	if hits, _ = st.Search("定例", 10); len(hits) != 1 || hits[0].Kind != "title" {
		t.Fatalf("title: %+v", hits)
	}
	if hits, _ = st.Search("  ", 10); len(hits) != 0 {
		t.Fatalf("blank query: %+v", hits)
	}
}

func TestSearchSeesEdits(t *testing.T) {
	st := Store{Root: t.TempDir()}
	a, _ := st.Create("x", time.Now())
	st.SaveTranscript(a.ID, Transcript{Segments: []Segment{{Text: "旧い"}}})
	if hits, _ := st.Search("新しい", 10); len(hits) != 0 {
		t.Fatalf("before: %+v", hits)
	}
	time.Sleep(10 * time.Millisecond) // a later modification time
	st.SaveTranscript(a.ID, Transcript{Segments: []Segment{{Text: "新しい"}}})
	if hits, _ := st.Search("新しい", 10); len(hits) != 1 {
		t.Fatalf("after edit: %+v", hits)
	}
}
