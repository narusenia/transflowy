// Package library stores recordings as one folder each:
// meta.json, transcript.json, audio and summary-*.md live side by side.
package library

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Status is where a recording is in the processing pipeline.
type Status string

const (
	StatusRecording  Status = "recording"
	StatusImporting  Status = "importing" // an imported file is being converted
	StatusQueued     Status = "queued"
	StatusProcessing Status = "processing"
	StatusDone       Status = "done"
	StatusFailed     Status = "failed"
)

// UnknownSpeaker marks a segment that has no speaker label.
const UnknownSpeaker = -1

// Meta is a recording's meta.json.
type Meta struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"createdAt"`
	Duration  float64   `json:"duration"` // seconds
	// Language is the user's choice for this recording; "" means detect.
	Language string `json:"language,omitempty"`
	// Detected is the language the engine reported, for display.
	Detected string `json:"detected,omitempty"`
	Engine   string `json:"engine,omitempty"`
	// ProcessedUntil is how many seconds of audio have a transcript, so a
	// restart continues where the pipeline stopped.
	ProcessedUntil float64 `json:"processedUntil,omitempty"`
	Status         Status  `json:"status"`
	Error          string  `json:"error,omitempty"`
}

// Segment is one line of a transcript. Original keeps the engine's text so an
// edit can be undone.
type Segment struct {
	Start    float64 `json:"start"`
	End      float64 `json:"end"`
	Speaker  int     `json:"speaker"` // UnknownSpeaker when not labelled
	Text     string  `json:"text"`
	Original string  `json:"original"`
}

// Transcript is a recording's transcript.json. Speakers maps a speaker number
// (as a string) to the name the user gave it.
type Transcript struct {
	Segments []Segment         `json:"segments"`
	Speakers map[string]string `json:"speakers,omitempty"`
}

// Recording is everything the page shows for one recording.
type Recording struct {
	Meta       Meta       `json:"meta"`
	Transcript Transcript `json:"transcript"`
	Summaries  []string   `json:"summaries"` // template names, from summary-<name>.md
}

// Store reads and writes recordings under Root.
type Store struct{ Root string }

var ErrNotFound = errors.New("recording not found")

// Create makes the folder for a new recording and writes its meta.json.
func (s Store) Create(title string, at time.Time) (Meta, error) {
	base := at.Format("2006-01-02_1504") + "_" + sanitize(title)
	id := base
	for i := 2; ; i++ {
		if err := os.MkdirAll(s.Root, 0o755); err != nil {
			return Meta{}, err
		}
		if err := os.Mkdir(filepath.Join(s.Root, id), 0o755); err == nil {
			break
		} else if !errors.Is(err, os.ErrExist) {
			return Meta{}, err
		}
		id = fmt.Sprintf("%s-%d", base, i)
	}
	m := Meta{ID: id, Title: title, CreatedAt: at, Status: StatusRecording}
	return m, s.SaveMeta(m)
}

func (s Store) dir(id string) (string, error) {
	// ids come from the page: refuse anything that is not a plain folder name.
	if id == "" || id != filepath.Base(id) || strings.HasPrefix(id, ".") {
		return "", ErrNotFound
	}
	return filepath.Join(s.Root, id), nil
}

// Path returns a recording's folder.
func (s Store) Path(id string) (string, error) { return s.dir(id) }

// writeMu serializes read-modify-write of meta.json and transcript.json: the
// recorder, the importer and the pipeline all update the same recording.
var writeMu sync.Mutex

// UpdateMeta applies fn to a recording's meta and saves it.
func (s Store) UpdateMeta(id string, fn func(*Meta)) (Meta, error) {
	writeMu.Lock()
	defer writeMu.Unlock()
	r, err := s.Get(id)
	if err != nil {
		return Meta{}, err
	}
	m := r.Meta
	fn(&m)
	return m, s.SaveMeta(m)
}

// UpdateTranscript applies fn to a recording's transcript and saves it.
func (s Store) UpdateTranscript(id string, fn func(*Transcript)) error {
	writeMu.Lock()
	defer writeMu.Unlock()
	r, err := s.Get(id)
	if err != nil {
		return err
	}
	t := r.Transcript
	fn(&t)
	return s.SaveTranscript(id, t)
}

// SaveMeta writes meta.json.
func (s Store) SaveMeta(m Meta) error { return s.write(m.ID, "meta.json", m) }

// SaveTranscript writes transcript.json.
func (s Store) SaveTranscript(id string, t Transcript) error {
	return s.write(id, "transcript.json", t)
}

// write replaces the file atomically so a crash never leaves half a JSON file.
func (s Store) write(id, name string, v any) error {
	d, err := s.dir(id)
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(d, name+".*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), filepath.Join(d, name))
}

// Get loads one recording. A missing transcript is an empty one.
func (s Store) Get(id string) (Recording, error) {
	d, err := s.dir(id)
	if err != nil {
		return Recording{}, err
	}
	var r Recording
	if err := read(filepath.Join(d, "meta.json"), &r.Meta); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Recording{}, ErrNotFound
		}
		return Recording{}, err
	}
	if err := read(filepath.Join(d, "transcript.json"), &r.Transcript); err != nil && !errors.Is(err, os.ErrNotExist) {
		return Recording{}, err
	}
	if r.Transcript.Segments == nil {
		r.Transcript.Segments = []Segment{}
	}
	files, _ := filepath.Glob(filepath.Join(d, "summary-*.md"))
	r.Summaries = []string{}
	for _, f := range files {
		name := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(f), "summary-"), ".md")
		r.Summaries = append(r.Summaries, name)
	}
	sort.Strings(r.Summaries)
	return r, nil
}

// List returns every recording's meta, newest first. Folders without a
// readable meta.json are skipped: the user may keep other things in the root.
func (s Store) List() ([]Meta, error) {
	entries, err := os.ReadDir(s.Root)
	if errors.Is(err, os.ErrNotExist) {
		return []Meta{}, nil
	}
	if err != nil {
		return nil, err
	}
	metas := []Meta{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		var m Meta
		if read(filepath.Join(s.Root, e.Name(), "meta.json"), &m) == nil {
			m.ID = e.Name()
			metas = append(metas, m)
		}
	}
	sort.Slice(metas, func(i, j int) bool { return metas[i].CreatedAt.After(metas[j].CreatedAt) })
	return metas, nil
}

func read(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

// sanitize makes a title safe as part of a folder name.
func sanitize(title string) string {
	r := strings.NewReplacer("/", "-", "\\", "-", ":", "-", "\x00", "")
	t := strings.TrimSpace(strings.TrimLeft(r.Replace(title), "."))
	if t == "" {
		return "録音"
	}
	if rs := []rune(t); len(rs) > 40 {
		t = string(rs[:40])
	}
	return t
}
