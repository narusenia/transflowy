package library

import (
	"os"
	"path/filepath"
	"strings"
)

// Hit is one search result. Time is where a transcript hit is, in seconds.
type Hit struct {
	ID       string  `json:"id"`
	Title    string  `json:"title"`
	Kind     string  `json:"kind"` // "title", "transcript" or "summary"
	Snippet  string  `json:"snippet"`
	Time     float64 `json:"time,omitempty"`
	Template string  `json:"template,omitempty"` // summary hits
}

const hitsPerRecording = 3

// Search looks for q (case-insensitive) in titles, transcripts and summaries,
// newest recording first, at most limit hits. It scans the files on every
// call: ponytail: fine for hundreds of recordings, add an index when it is slow.
func (s Store) Search(q string, limit int) ([]Hit, error) {
	q = strings.ToLower(strings.TrimSpace(q))
	metas, err := s.List()
	if err != nil || q == "" {
		return []Hit{}, err
	}
	has := func(text string) bool { return strings.Contains(strings.ToLower(text), q) }
	hits := []Hit{}
	for _, m := range metas {
		if len(hits) >= limit {
			break
		}
		n := 0
		add := func(h Hit) {
			if n < hitsPerRecording {
				h.ID, h.Title = m.ID, m.Title
				hits = append(hits, h)
				n++
			}
		}
		if has(m.Title) {
			add(Hit{Kind: "title", Snippet: m.Title})
		}
		r, err := s.Get(m.ID)
		if err != nil {
			continue
		}
		for _, seg := range r.Transcript.Segments {
			if has(seg.Text) {
				add(Hit{Kind: "transcript", Snippet: seg.Text, Time: seg.Start})
			}
		}
		d, _ := s.Path(m.ID)
		for _, name := range r.Summaries {
			b, _ := os.ReadFile(filepath.Join(d, "summary-"+name+".md"))
			for _, line := range strings.Split(string(b), "\n") {
				if has(line) {
					add(Hit{Kind: "summary", Snippet: strings.TrimSpace(line), Template: name})
					break
				}
			}
		}
	}
	return hits[:min(len(hits), limit)], nil
}
