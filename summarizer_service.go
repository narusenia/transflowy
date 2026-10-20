package main

import (
	"context"
	"errors"
	"fmt"

	"transflowy/internal/library"
	"transflowy/internal/settings"
	"transflowy/internal/summarize"
)

// Summarizer is callable from the page.
type Summarizer struct {
	cfg     *settings.Service
	engines *Engines
	queue   *summarize.Queue
}

// Templates lists the built-in and the user's summary templates.
func (s *Summarizer) Templates() ([]summarize.Template, error) {
	cur, err := s.cfg.Get()
	return summarize.Templates(cur.LibraryDir), err
}

// Summarize writes a summary of a finished transcript with a template, keeping
// the summaries made with other templates. A summary of the same template is
// replaced. It is also the retry after a failed summary.
func (s *Summarizer) Summarize(id, template string) error {
	cur, err := s.cfg.Get()
	if err != nil {
		return err
	}
	rec, err := library.Store{Root: cur.LibraryDir}.Get(id)
	if err != nil {
		return err
	}
	if rec.Meta.Status != library.StatusDone && rec.Meta.Status != library.StatusSummarizing {
		return errors.New("the transcript is not finished")
	}
	found := false
	for _, t := range summarize.Templates(cur.LibraryDir) {
		found = found || t.Name == template
	}
	if !found {
		return fmt.Errorf("no template %q", template)
	}
	s.queue.Enqueue(id, template)
	return nil
}

// DetectOllama lists the models of a local Ollama; empty when it is not running.
// Use baseUrl "http://localhost:11434/v1" with the openai-compat summarize provider.
func (s *Summarizer) DetectOllama() []string {
	return summarize.DetectOllama(context.Background(), s.engines.client(), "http://localhost:11434")
}
