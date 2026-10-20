package main

import (
	"context"
	"sync"
	"time"

	"github.com/egoist/mygo"

	"transflowy/internal/models"
)

// ModelProgress is sent while a model downloads. Kind is "whisper" (the
// transcription model) or "speaker" (the two speaker diarization models).
type ModelProgress struct {
	Kind  string `json:"kind"`
	State string `json:"state"` // downloading, done, failed
	Done  int64  `json:"done"`
	Total int64  `json:"total"`
	Error string `json:"error,omitempty"`
}

// ModelChanged fires during a download.
var ModelChanged = mygo.NewEvent[ModelProgress]("modelChanged")

// ModelStatus tells the page which local models are there.
type ModelStatus struct {
	Present            bool `json:"present"`
	Downloading        bool `json:"downloading"`
	SpeakerPresent     bool `json:"speakerPresent"`
	SpeakerDownloading bool `json:"speakerDownloading"`
}

// Models is callable from the page.
type Models struct {
	dir     string
	onReady func() // recordings that waited for the model are processed now

	mu          sync.Mutex
	downloading map[string]bool
}

var speakerModels = []models.Model{models.SpeakerSegmentation, models.SpeakerEmbedding}

func speakerPresent(dir string) bool {
	return models.SpeakerSegmentation.Present(dir) && models.SpeakerEmbedding.Present(dir)
}

// Status reports which models are downloaded or downloading.
func (m *Models) Status() ModelStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	return ModelStatus{
		Present: models.Whisper.Present(m.dir), Downloading: m.downloading["whisper"],
		SpeakerPresent: speakerPresent(m.dir), SpeakerDownloading: m.downloading["speaker"],
	}
}

// Download starts the transcription model's download in the background and
// returns at once; progress arrives as modelChanged events. A second call
// while one runs does nothing.
func (m *Models) Download() { m.fetch("whisper", []models.Model{models.Whisper}) }

// DownloadSpeaker does the same for the speaker diarization models (about 45 MB).
func (m *Models) DownloadSpeaker() { m.fetch("speaker", speakerModels) }

func (m *Models) fetch(kind string, files []models.Model) {
	m.mu.Lock()
	missing := []models.Model{}
	for _, f := range files {
		if !f.Present(m.dir) {
			missing = append(missing, f)
		}
	}
	if m.downloading[kind] || len(missing) == 0 {
		m.mu.Unlock()
		return
	}
	if m.downloading == nil {
		m.downloading = map[string]bool{}
	}
	m.downloading[kind] = true
	m.mu.Unlock()
	go func() {
		var last time.Time
		var err error
		for _, f := range missing {
			err = f.Download(context.Background(), m.dir, func(done, total int64) {
				if time.Since(last) > 250*time.Millisecond { // the page does not need every chunk
					last = time.Now()
					// With several files the bar is per file: the page only needs it to move.
					_ = ModelChanged.Broadcast(ModelProgress{Kind: kind, State: "downloading", Done: done, Total: total})
				}
			})
			if err != nil {
				break
			}
		}
		m.mu.Lock()
		m.downloading[kind] = false
		m.mu.Unlock()
		if err != nil {
			_ = ModelChanged.Broadcast(ModelProgress{Kind: kind, State: "failed", Error: err.Error()})
			return
		}
		_ = ModelChanged.Broadcast(ModelProgress{Kind: kind, State: "done"})
		m.onReady()
	}()
}
