package app

import (
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"

	"transflowy/internal/audio"
	"transflowy/internal/library"
	"transflowy/internal/pipeline"
	"transflowy/internal/settings"
	"transflowy/locales"
)

// importExts are the files afconvert reads: audio, and the audio of video.
var importExts = []string{"m4a", "mp3", "wav", "aac", "caf", "aiff", "aif", "flac", "mp4", "mov", "m4v"}

// LibraryChanged fires with a recording's id when an import finishes.
var LibraryChanged = mygo.NewEvent[string]("libraryChanged")

// ImportProblem is sent when a file cannot be imported.
type ImportProblem struct {
	Name    string `json:"name"`
	Message string `json:"message"`
}

// ImportFailed fires for an unsupported or unreadable file.
var ImportFailed = mygo.NewEvent[ImportProblem]("importFailed")

// Importer is callable from the page. Files dropped on the window or the Dock
// icon reach importFiles on the Go side.
type Importer struct {
	cfg   *settings.Service
	queue *pipeline.Queue
}

// ImportDialog asks for files and imports them.
func (im *Importer) ImportDialog() error {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Title:    locales.ImportTitle(),
		Multiple: true,
		Filters:  []mygo.FileFilter{{Name: "Audio / Video", Extensions: importExts}},
	})
	if err != nil {
		return err
	}
	im.importFiles(paths)
	return nil
}

func supported(path string) bool {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
	for _, e := range importExts {
		if e == ext {
			return true
		}
	}
	return false
}

// importFiles imports each file in the background. The originals are only read.
func (im *Importer) importFiles(paths []string) {
	s, err := im.cfg.Get()
	if err != nil {
		log.Println("import:", err)
		return
	}
	st := library.Store{Root: s.LibraryDir}
	for _, p := range paths {
		name := filepath.Base(p)
		if !supported(p) {
			_ = ImportFailed.Broadcast(ImportProblem{Name: name, Message: locales.ImportUnsupported(name)})
			continue
		}
		go func() {
			if err := importOne(st, im.queue, p); err != nil {
				log.Println("import:", name, err)
				_ = ImportFailed.Broadcast(ImportProblem{Name: name, Message: locales.ImportFailed(name)})
			}
		}()
	}
}

func importOne(st library.Store, q *pipeline.Queue, path string) error {
	title := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	m, err := st.Create(title, time.Now())
	if err != nil {
		return err
	}
	m.Status = library.StatusImporting
	if err := st.SaveMeta(m); err != nil {
		return err
	}
	fail := func(err error) error {
		m.Status, m.Error = library.StatusFailed, err.Error()
		_ = st.SaveMeta(m)
		_ = LibraryChanged.Broadcast(m.ID)
		return err
	}
	dir, _ := st.Path(m.ID)
	wav := filepath.Join(dir, wavName)
	if err := audio.ToWAV16k(path, wav); err != nil {
		return fail(err)
	}
	n, err := audio.PCMSamples(wav)
	if err != nil {
		return fail(err)
	}
	m.Duration = float64(n) / 16000
	m.Status = library.StatusQueued
	if err := st.SaveMeta(m); err != nil {
		return err
	}
	_ = LibraryChanged.Broadcast(m.ID)
	q.Enqueue(m.ID, "", false)
	finalizeAudio(st, m.ID)
	return nil
}
