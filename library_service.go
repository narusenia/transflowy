package main

import (
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
