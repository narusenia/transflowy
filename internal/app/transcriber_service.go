package app

import (
	"errors"
	"regexp"

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

var langCode = regexp.MustCompile(`^[a-z]{2,3}$`)

// SetLanguage fixes a recording's language (ISO 639-1, "ja", "en"); "" detects.
// It applies to the next processing: call Reprocess to redo the transcript.
func (t *Transcriber) SetLanguage(id, lang string) error {
	if lang != "" && !langCode.MatchString(lang) {
		return errors.New("language must be a code like ja or en")
	}
	s, err := t.cfg.Get()
	if err != nil {
		return err
	}
	_, err = library.Store{Root: s.LibraryDir}.UpdateMeta(id, func(m *library.Meta) { m.Language = lang })
	return err
}

// Diarize tells the voices apart again in a transcribed recording with the
// local models; the text stays as it is. speakers is how many people talk, or
// 0 to find out (a wrong count is worse than none: use it when it is known).
func (t *Transcriber) Diarize(id string, speakers int) error {
	if speakers < 0 || speakers > 20 {
		return errors.New("speakers must be between 0 and 20")
	}
	s, err := t.cfg.Get()
	if err != nil {
		return err
	}
	rec, err := (library.Store{Root: s.LibraryDir}).Get(id)
	if err != nil {
		return err
	}
	if rec.Meta.Status != library.StatusDone {
		return errors.New("the transcript is not finished")
	}
	t.queue.Diarize(id, speakers)
	return nil
}
