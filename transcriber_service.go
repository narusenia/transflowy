package main

import (
	"transflowy/internal/library"
	"transflowy/internal/pipeline"
	"transflowy/internal/settings"
)

// Transcriber is callable from the page.
type Transcriber struct {
	cfg   *settings.Service
	queue *pipeline.Queue
}

// Reprocess transcribes a recording again from the start. kind names the
// engine for this run ("local", "openrouter", "openai-compat"); "" uses the
// setting. It is also the retry after a failure.
func (t *Transcriber) Reprocess(id, kind string) error {
	s, err := t.cfg.Get()
	if err != nil {
		return err
	}
	if _, err := (library.Store{Root: s.LibraryDir}).Get(id); err != nil {
		return err
	}
	_, err = library.Store{Root: s.LibraryDir}.UpdateMeta(id, func(m *library.Meta) {
		m.Status, m.Error = library.StatusQueued, ""
	})
	if err != nil {
		return err
	}
	t.queue.Enqueue(id, kind, true)
	return nil
}
