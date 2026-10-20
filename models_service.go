package main

import (
	"context"
	"sync"
	"time"

	"github.com/egoist/mygo"

	"transflowy/internal/models"
)

// ModelProgress is sent while the local model downloads.
type ModelProgress struct {
	State string `json:"state"` // downloading, done, failed
	Done  int64  `json:"done"`
	Total int64  `json:"total"`
	Error string `json:"error,omitempty"`
}

// ModelChanged fires during a download.
var ModelChanged = mygo.NewEvent[ModelProgress]("modelChanged")

// ModelStatus tells the page whether the local model is there.
type ModelStatus struct {
	Present     bool `json:"present"`
	Downloading bool `json:"downloading"`
}

// Models is callable from the page.
type Models struct {
	dir     string
	onReady func() // recordings that waited for the model are processed now

	mu          sync.Mutex
	downloading bool
}

// Status reports whether the model is downloaded or downloading.
func (m *Models) Status() ModelStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	return ModelStatus{Present: models.Whisper.Present(m.dir), Downloading: m.downloading}
}

// Download starts the download in the background and returns at once;
// progress arrives as modelChanged events. A second call while one runs does
// nothing.
func (m *Models) Download() {
	m.mu.Lock()
	if m.downloading || models.Whisper.Present(m.dir) {
		m.mu.Unlock()
		return
	}
	m.downloading = true
	m.mu.Unlock()
	go func() {
		var last time.Time
		err := models.Whisper.Download(context.Background(), m.dir, func(done, total int64) {
			if time.Since(last) > 250*time.Millisecond { // the page does not need every chunk
				last = time.Now()
				_ = ModelChanged.Broadcast(ModelProgress{State: "downloading", Done: done, Total: total})
			}
		})
		m.mu.Lock()
		m.downloading = false
		m.mu.Unlock()
		if err != nil {
			_ = ModelChanged.Broadcast(ModelProgress{State: "failed", Error: err.Error()})
			return
		}
		_ = ModelChanged.Broadcast(ModelProgress{State: "done"})
		m.onReady()
	}()
}
