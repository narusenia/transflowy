package library

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
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

// doc is one searchable line; lower is text lowercased once, when indexed.
type doc struct {
	Hit
	lower string
}

// indexed is a recording's searchable text, valid while its files keep the
// same modification times (stamp).
type indexed struct {
	stamp string
	meta  Meta
	docs  []doc
}

// index keeps every recording's text in memory, so a search reads no files
// but stats a few per recording. ponytail: no SQLite FTS; a linear scan of
// memory is fine for thousands of recordings, add FTS if it is not.
var index = struct {
	sync.Mutex
	m map[string]*indexed
}{m: map[string]*indexed{}}

func stampOf(dir string) string {
	var b strings.Builder
	stat := func(p string) {
		if fi, err := os.Stat(p); err == nil {
			b.WriteString(fi.ModTime().Format(time.RFC3339Nano))
		}
		b.WriteByte('|')
	}
	stat(filepath.Join(dir, "meta.json"))
	stat(filepath.Join(dir, "transcript.json"))
	files, _ := filepath.Glob(filepath.Join(dir, "summary-*.md"))
	for _, f := range files {
		b.WriteString(filepath.Base(f))
		stat(f)
	}
	return b.String()
}

// load returns the index entry of a recording, rebuilding it when stale.
func (s Store) load(id string) (*indexed, bool) {
	d, err := s.dir(id)
	if err != nil {
		return nil, false
	}
	key, stamp := d, stampOf(d)
	index.Lock()
	e := index.m[key]
	index.Unlock()
	if e != nil && e.stamp == stamp {
		return e, true
	}
	r, err := s.Get(id)
	if err != nil {
		return nil, false
	}
	e = &indexed{stamp: stamp, meta: r.Meta}
	add := func(h Hit) { e.docs = append(e.docs, doc{h, strings.ToLower(h.Snippet)}) }
	add(Hit{Kind: "title", Snippet: r.Meta.Title})
	for _, seg := range r.Transcript.Segments {
		add(Hit{Kind: "transcript", Snippet: seg.Text, Time: seg.Start})
	}
	for _, name := range r.Summaries {
		b, _ := os.ReadFile(filepath.Join(d, "summary-"+name+".md"))
		for _, line := range strings.Split(string(b), "\n") {
			if line = strings.TrimSpace(line); line != "" {
				add(Hit{Kind: "summary", Snippet: line, Template: name})
			}
		}
	}
	index.Lock()
	index.m[key] = e
	index.Unlock()
	return e, true
}

// Search looks for q (case-insensitive) in titles, transcripts and summaries,
// newest recording first, at most limit hits (a summary gives one hit each).
func (s Store) Search(q string, limit int) ([]Hit, error) {
	q = strings.ToLower(strings.TrimSpace(q))
	entries, err := os.ReadDir(s.Root)
	if err != nil || q == "" {
		return []Hit{}, nil
	}
	var all []*indexed
	for _, de := range entries {
		if de.IsDir() {
			if e, ok := s.load(de.Name()); ok {
				e.meta.ID = de.Name()
				all = append(all, e)
			}
		}
	}
	sort.Slice(all, func(i, j int) bool { return all[i].meta.CreatedAt.After(all[j].meta.CreatedAt) })
	hits := []Hit{}
	for _, e := range all {
		if len(hits) >= limit {
			break
		}
		n, seen := 0, map[string]bool{}
		for _, d := range e.docs {
			if n >= hitsPerRecording {
				break
			}
			if d.Kind == "summary" && seen[d.Template] || !strings.Contains(d.lower, q) {
				continue
			}
			seen[d.Template] = true
			h := d.Hit
			h.ID, h.Title = e.meta.ID, e.meta.Title
			hits = append(hits, h)
			n++
		}
	}
	return hits[:min(len(hits), limit)], nil
}
