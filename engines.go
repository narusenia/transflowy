package main

import (
	"bufio"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"transflowy/internal/diarize"
	"transflowy/internal/library"
	"transflowy/internal/models"
	"transflowy/internal/netguard"
	"transflowy/internal/pipeline"
	"transflowy/internal/settings"
	"transflowy/internal/summarize"
	"transflowy/internal/transcribe"
	"transflowy/locales"
)

// Engines builds the transcription engine the settings name. Every request
// goes through one client that enforces local-only.
type Engines struct {
	cfg      *settings.Service
	server   *transcribe.Server
	modelDir string
}

func (e *Engines) client() *http.Client {
	return netguard.Client(func() bool {
		s, _ := e.cfg.Get()
		return s.LocalOnly
	})
}

// key reads a provider's key from the keychain. OPENROUTER_API_KEY (from a
// .env under `mygo dev`) is the fallback for development.
func (e *Engines) key(kind string) func() (string, error) {
	return func() (string, error) {
		k, err := settings.ReadKey(e.cfg, kind)
		if k == "" && kind == settings.KindOpenRouter {
			k = os.Getenv("OPENROUTER_API_KEY")
		}
		return k, err
	}
}

// Build is the pipeline's engine factory. kind overrides the setting for one
// recording ("reprocess with another engine"); "" uses the setting.
func (e *Engines) Build(kind string) (pipeline.Job, error) {
	s, err := e.cfg.Get()
	if err != nil {
		return pipeline.Job{}, err
	}
	p := s.Transcribe
	if kind != "" && kind != p.Kind {
		p = settings.Provider{Kind: kind}
	}
	notReady := func(msg string) (pipeline.Job, error) { return pipeline.Job{}, pipeline.NotReadyError{Msg: msg} }

	switch p.Kind {
	case settings.KindLocal:
		if _, err := transcribe.FindBinary(); err != nil {
			return notReady(err.Error())
		}
		if !models.Whisper.Present(e.modelDir) {
			return notReady(locales.ErrNoModel())
		}
		return pipeline.Job{Engine: transcribe.Local{Server: e.server, Client: e.client()}, Params: transcribe.LocalParams}, nil

	case settings.KindOpenRouter:
		if s.LocalOnly {
			return notReady(locales.ErrLocalOnly())
		}
		key := e.key(p.Kind)
		if k, _ := key(); k == "" {
			return notReady(locales.ErrNoKey())
		}
		return pipeline.Job{
			Engine:  transcribe.OpenRouter{Client: e.client(), Key: key, Model: p.Model},
			Params:  transcribe.CloudParams,
			Diarize: true,
		}, nil

	case settings.KindOpenAICompat:
		u, err := url.Parse(strings.TrimRight(p.BaseURL, "/"))
		if err != nil || u.Host == "" {
			return notReady(locales.ErrNoBaseURL())
		}
		if !netguard.Allowed(s.LocalOnly, u.Hostname()) {
			return notReady(locales.ErrLocalOnly())
		}
		w := transcribe.Whisper{Client: e.client(), URL: u.String() + "/audio/transcriptions", Model: p.Model, Key: e.key(p.Kind)}
		return pipeline.Job{Engine: w, Params: transcribe.LocalParams}, nil
	}
	return pipeline.Job{}, os.ErrInvalid
}

// Chat is the summary queue's model factory: the summarize provider of the
// settings, through the same local-only client.
func (e *Engines) Chat() (summarize.Chat, error) {
	s, err := e.cfg.Get()
	if err != nil {
		return summarize.Chat{}, err
	}
	p := s.Summarize
	notReady := func(msg string) (summarize.Chat, error) {
		return summarize.Chat{}, pipeline.NotReadyError{Msg: msg}
	}
	if p.Model == "" {
		return notReady(locales.ErrNoSummaryModel())
	}
	switch p.Kind {
	case settings.KindOpenRouter:
		if s.LocalOnly {
			return notReady(locales.ErrLocalOnly())
		}
		key := e.key(p.Kind)
		if k, _ := key(); k == "" {
			return notReady(locales.ErrNoKey())
		}
		return summarize.Chat{Client: e.client(), BaseURL: "https://openrouter.ai/api/v1", Key: key, Model: p.Model}, nil
	case settings.KindOpenAICompat: // Ollama, LM Studio, ...; a key is optional
		u, err := url.Parse(strings.TrimRight(p.BaseURL, "/"))
		if err != nil || u.Host == "" {
			return notReady(locales.ErrNoBaseURL())
		}
		if !netguard.Allowed(s.LocalOnly, u.Hostname()) {
			return notReady(locales.ErrLocalOnly())
		}
		return summarize.Chat{Client: e.client(), BaseURL: u.String(), Key: e.key(p.Kind), Model: p.Model}, nil
	}
	return summarize.Chat{}, os.ErrInvalid
}

// Diarizer is the pipeline's factory for local speaker diarization. auto is
// the run after a transcription: the setting decides, and a missing model
// just skips it (the pipeline does not ask when the engine diarized itself). A run the user asked for explains what is missing.
func (e *Engines) Diarizer(auto bool) (func([]int16) ([]diarize.Turn, error), error) {
	s, err := e.cfg.Get()
	if err != nil {
		return nil, err
	}
	if auto && !s.LocalDiarization {
		return nil, nil
	}
	if !diarize.Supported {
		return nil, pipeline.NotReadyError{Msg: locales.ErrDiarizeUnsupported()}
	}
	if !speakerPresent(e.modelDir) {
		return nil, pipeline.NotReadyError{Msg: locales.ErrNoSpeakerModel()}
	}
	cfg := diarize.Config{Segmentation: models.SpeakerSegmentation.Path(e.modelDir), Embedding: models.SpeakerEmbedding.Path(e.modelDir)}
	return func(pcm []int16) ([]diarize.Turn, error) { return diarize.Run(cfg, pcm) }, nil
}

// phrases reads dictionary.txt in the library: one name or term per line.
func phrases(cfg *settings.Service) func() []string {
	return func() []string {
		s, err := cfg.Get()
		if err != nil {
			return nil
		}
		f, err := os.Open(filepath.Join(s.LibraryDir, "dictionary.txt"))
		if err != nil {
			return nil
		}
		defer f.Close()
		var out []string
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			if t := strings.TrimSpace(sc.Text()); t != "" && !strings.HasPrefix(t, "#") {
				out = append(out, t)
			}
		}
		return out
	}
}

// resumeAll queues every recording that is waiting for processing: after a
// start, a finished model download or a settings change.
func resumeAll(cfg *settings.Service, q *pipeline.Queue, sum *summarize.Queue) {
	s, err := cfg.Get()
	if err != nil {
		return
	}
	metas, _ := library.Store{Root: s.LibraryDir}.List()
	for _, m := range metas {
		if m.Status == library.StatusQueued || m.Status == library.StatusProcessing || m.Status == library.StatusDiarizing {
			q.Enqueue(m.ID, "", false)
		}
		if m.Status == library.StatusSummarizing {
			sum.Enqueue(m.ID, "")
		}
	}
}
