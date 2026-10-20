package main

import (
	"github.com/egoist/mygo"

	"transflowy/internal/library"
	"transflowy/internal/settings"
)

// Library is callable from the page: it reads recordings from the folder the
// settings name. Creating and changing recordings is done on the Go side.
type Library struct{ cfg *settings.Service }

func (l *Library) store() (library.Store, error) {
	s, err := l.cfg.Get()
	return library.Store{Root: s.LibraryDir}, err
}

// List returns every recording's meta, newest first.
func (l *Library) List() ([]library.Meta, error) {
	st, err := l.store()
	if err != nil {
		return nil, err
	}
	return st.List()
}

// Get returns one recording with its transcript.
func (l *Library) Get(id string) (library.Recording, error) {
	st, err := l.store()
	if err != nil {
		return library.Recording{}, err
	}
	return st.Get(id)
}

// Summary returns the Markdown of a recording's summary made with a template.
func (l *Library) Summary(id, template string) (string, error) {
	st, err := l.store()
	if err != nil {
		return "", err
	}
	return st.Summary(id, template)
}

// Search finds recordings by title, transcript and summary text.
func (l *Library) Search(query string) ([]library.Hit, error) {
	st, err := l.store()
	if err != nil {
		return nil, err
	}
	return st.Search(query, 50)
}

// ChooseFolder asks for a folder and returns it; "" when canceled.
func (l *Library) ChooseFolder() (string, error) {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Directory: true, CreateDirectories: true})
	if err != nil || len(paths) == 0 {
		return "", err
	}
	return paths[0], nil
}

// SetTitle renames a recording.
func (l *Library) SetTitle(id, title string) error {
	st, err := l.store()
	if err != nil {
		return err
	}
	return st.SetTitle(id, title)
}

// EditSegment changes the text of one transcript line. start is the line's
// start time as the page has it; the original engine text is kept.
func (l *Library) EditSegment(id string, index int, start float64, text string) error {
	st, err := l.store()
	if err != nil {
		return err
	}
	return st.EditSegment(id, index, start, text)
}

// ResetSegment puts the engine's original text back into one line.
func (l *Library) ResetSegment(id string, index int, start float64) error {
	st, err := l.store()
	if err != nil {
		return err
	}
	return st.ResetSegment(id, index, start)
}

// Replace changes every occurrence of find in a recording's transcript and
// returns how many lines changed.
func (l *Library) Replace(id, find, with string) (int, error) {
	st, err := l.store()
	if err != nil {
		return 0, err
	}
	return st.Replace(id, find, with)
}

// AddToDictionary adds a word to dictionary.txt, which the engines get.
func (l *Library) AddToDictionary(word string) error {
	st, err := l.store()
	if err != nil {
		return err
	}
	return st.AddToDictionary(word)
}
