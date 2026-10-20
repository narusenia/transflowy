package library

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ErrStaleSegment means the transcript changed under the page (a reprocess
// replaced it): the segment at that index is not the one the user edited.
var ErrStaleSegment = errors.New("the transcript changed; reload it")

// SetTitle renames a recording. The folder keeps its name.
func (s Store) SetTitle(id, title string) error {
	_, err := s.UpdateMeta(id, func(m *Meta) { m.Title = strings.TrimSpace(title) })
	return err
}

// EditSegment replaces the text of one line. start is the line's start time as
// the page saw it, to catch an index that now points at another line.
func (s Store) EditSegment(id string, index int, start float64, text string) error {
	return s.editSegment(id, index, start, func(seg *Segment) { seg.Text = strings.TrimSpace(text) })
}

// ResetSegment puts the engine's original text back.
func (s Store) ResetSegment(id string, index int, start float64) error {
	return s.editSegment(id, index, start, func(seg *Segment) { seg.Text = seg.Original })
}

func (s Store) editSegment(id string, index int, start float64, fn func(*Segment)) error {
	var err error
	if e := s.UpdateTranscript(id, func(t *Transcript) {
		if index < 0 || index >= len(t.Segments) || t.Segments[index].Start != start {
			err = ErrStaleSegment
			return
		}
		fn(&t.Segments[index])
	}); e != nil {
		return e
	}
	return err
}

// SetSpeakers saves the names the user gave to speakers: speaker number (as a
// string) to name. A blank name goes back to the default "話者N".
func (s Store) SetSpeakers(id string, names map[string]string) error {
	return s.UpdateTranscript(id, func(t *Transcript) {
		t.Speakers = map[string]string{}
		for k, v := range names {
			if v = strings.TrimSpace(v); v != "" {
				t.Speakers[k] = v
			}
		}
		if len(t.Speakers) == 0 {
			t.Speakers = nil
		}
	})
}

// Replace changes every occurrence of find in the transcript's lines and
// returns how many lines changed.
func (s Store) Replace(id, find, with string) (int, error) {
	if find == "" {
		return 0, nil
	}
	n := 0
	err := s.UpdateTranscript(id, func(t *Transcript) {
		for i := range t.Segments {
			if strings.Contains(t.Segments[i].Text, find) {
				t.Segments[i].Text = strings.ReplaceAll(t.Segments[i].Text, find, with)
				n++
			}
		}
	})
	return n, err
}

// AddToDictionary appends a word to dictionary.txt in the library, unless the
// file has it already. The transcription engines get that file's words.
func (s Store) AddToDictionary(word string) error {
	word = strings.TrimSpace(word)
	if word == "" || strings.ContainsAny(word, "\r\n") {
		return nil
	}
	if err := os.MkdirAll(s.Root, 0o755); err != nil {
		return err
	}
	path := filepath.Join(s.Root, "dictionary.txt")
	b, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(line) == word {
			return nil
		}
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if len(b) > 0 && !strings.HasSuffix(string(b), "\n") {
		word = "\n" + word
	}
	_, err = f.WriteString(word + "\n")
	return err
}

// AppendSegments adds lines to the transcript and keeps it in time order: two
// tracks (microphone and system audio) are transcribed side by side.
func (s Store) AppendSegments(id string, segs []Segment) error {
	return s.UpdateTranscript(id, func(t *Transcript) {
		t.Segments = append(t.Segments, segs...)
		sort.SliceStable(t.Segments, func(i, j int) bool { return t.Segments[i].Start < t.Segments[j].Start })
	})
}
