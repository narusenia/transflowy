package library

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestStoreRoundTrip(t *testing.T) {
	s := Store{Root: filepath.Join(t.TempDir(), "root")} // Root does not exist yet
	at := time.Date(2026, 10, 20, 10, 30, 0, 0, time.UTC)

	a, err := s.Create("週次定例", at)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := s.Create("週次定例", at) // same title and minute must not collide
	if a.ID != "2026-10-20_1030_週次定例" || b.ID != a.ID+"-2" {
		t.Fatalf("ids: %q %q", a.ID, b.ID)
	}

	tr := Transcript{Segments: []Segment{{Start: 0, End: 1.5, Speaker: UnknownSpeaker, Text: "田中", Original: "田仲"}}}
	if err := s.SaveTranscript(a.ID, tr); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(s.Root, a.ID, "summary-議事録.md"), []byte("x"), 0o644)

	r, err := s.Get(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if r.Transcript.Segments[0].Original != "田仲" || len(r.Summaries) != 1 || r.Summaries[0] != "議事録" {
		t.Fatalf("got %+v", r)
	}

	os.Mkdir(filepath.Join(s.Root, "not-a-recording"), 0o755) // skipped by List
	metas, _ := s.List()
	if len(metas) != 2 {
		t.Fatalf("list: %d", len(metas))
	}

	for _, id := range []string{"", "../x", ".hidden", "missing"} {
		if _, err := s.Get(id); !errors.Is(err, ErrNotFound) {
			t.Errorf("Get(%q) = %v, want ErrNotFound", id, err)
		}
	}
}

func TestUpdateIsSerialized(t *testing.T) {
	s := Store{Root: t.TempDir()}
	m, _ := s.Create("x", time.Now())
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ { // concurrent writers must not lose each other's change
		wg.Add(2)
		go func() { defer wg.Done(); s.UpdateMeta(m.ID, func(m *Meta) { m.Duration++ }) }()
		go func() {
			defer wg.Done()
			s.UpdateTranscript(m.ID, func(t *Transcript) { t.Segments = append(t.Segments, Segment{}) })
		}()
	}
	wg.Wait()
	r, _ := s.Get(m.ID)
	if r.Meta.Duration != 50 || len(r.Transcript.Segments) != 50 {
		t.Fatalf("duration %v, segments %d", r.Meta.Duration, len(r.Transcript.Segments))
	}
}
