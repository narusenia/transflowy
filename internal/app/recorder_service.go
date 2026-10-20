package app

import (
	"errors"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/egoist/mygo"

	"transflowy/internal/audio"
	"transflowy/internal/library"
	"transflowy/internal/pipeline"
	"transflowy/internal/recorder"
	"transflowy/internal/settings"
	"transflowy/internal/systemaudio"
	"transflowy/locales"
)

// RecordingState is sent to the page while a recording runs and when it ends.
type RecordingState struct {
	Recording bool    `json:"recording"`
	ID        string  `json:"id"`
	Elapsed   float64 `json:"elapsed"` // seconds
	Level     float32 `json:"level"`   // 0..1
}

// RecordingChanged fires on start, stop and about five times a second while recording.
var RecordingChanged = mygo.NewEvent[RecordingState]("recordingChanged")

// RecorderWarning fires when a recording went on with less than was asked for.
var RecorderWarning = mygo.NewEvent[string]("recorderWarning")

const (
	systemWAVName = "system.wav"
	micM4AName    = "mic.m4a"
	systemM4AName = "system.m4a"
	wavName       = "recording.wav" // removed by the pipeline once transcribed and the m4a exists
	m4aName       = "audio.m4a"
)

// Recorder is callable from the page. The shortcut and the menu bar call the
// same methods on the Go side.
type Recorder struct {
	cfg   *settings.Service
	queue *pipeline.Queue // nil in tests
	rec   recorder.Recorder
	sys   systemaudio.Recorder // the second track, when the settings ask for it

	// onState is called with every state change, for the menu bar.
	onState func(RecordingState)

	mu   sync.Mutex
	id   string // recording in progress
	last string // last stopped recording: the only one Discard may delete
	quit chan struct{}
}

func (r *Recorder) store() (library.Store, error) {
	s, err := r.cfg.Get()
	return library.Store{Root: s.LibraryDir}, err
}

// Devices lists the input devices for the settings screen.
func (r *Recorder) Devices() ([]string, error) { return recorder.Devices() }

// Start begins a recording from the input device in the settings (the default one when none).
func (r *Recorder) Start() (library.Meta, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	st, err := r.store()
	if err != nil {
		return library.Meta{}, err
	}
	m, err := st.Create("", time.Now())
	if err != nil {
		return library.Meta{}, err
	}
	dir, _ := st.Path(m.ID)
	cur, _ := r.cfg.Get()
	if err := r.rec.Start(filepath.Join(dir, wavName), cur.InputDevice); err != nil {
		_ = os.RemoveAll(dir) // nothing was recorded: leave no empty folder
		return library.Meta{}, err
	}
	if cur.CaptureSystem && systemaudio.Supported {
		if err := r.sys.Start(filepath.Join(dir, systemWAVName)); err != nil {
			log.Println("system audio:", err)
			_ = RecorderWarning.Broadcast(locales.WarnSystemAudio(err.Error()))
		}
	}
	r.id = m.ID
	r.quit = make(chan struct{})
	go r.publish(r.quit, m.ID)
	r.emit(true, m.ID)
	r.enqueue(m.ID) // transcribing starts while the recording runs
	return m, nil
}

func (r *Recorder) publish(quit chan struct{}, id string) {
	t := time.NewTicker(200 * time.Millisecond)
	defer t.Stop()
	for n := 1; ; n++ {
		select {
		case <-t.C:
			if n%10 == 0 { // a crash loses at most this much of the system track
				r.sys.Flush()
			}
			r.emit(true, id)
		case <-quit:
			return
		}
	}
}

func (r *Recorder) emit(recording bool, id string) {
	st := RecordingState{Recording: recording, ID: id, Elapsed: r.rec.Elapsed(), Level: max(r.rec.Level(), r.sys.Level())}
	_ = RecordingChanged.Broadcast(st)
	if r.onState != nil {
		r.onState(st)
	}
}

// Stop ends the recording. The m4a is made in the background.
func (r *Recorder) Stop() (library.Meta, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.id == "" {
		return library.Meta{}, errors.New("not recording")
	}
	id := r.id
	close(r.quit)
	secs, stopErr := r.rec.Stop()
	if r.sys.Recording() {
		sysSecs, sysErr := r.sys.Stop()
		secs = max(secs, sysSecs)
		if stopErr == nil {
			stopErr = sysErr
		}
	}
	r.id, r.last = "", id
	r.emit(false, id)

	st, err := r.store()
	if err != nil {
		return library.Meta{}, err
	}
	m, err := st.UpdateMeta(id, func(m *library.Meta) {
		m.Duration = secs
		m.Status = library.StatusQueued
		if stopErr != nil { // keep what was written; the user sees why it may be short
			m.Error = stopErr.Error()
		}
	})
	if err != nil {
		return m, err
	}
	go finalizeAudio(st, id)
	r.enqueue(id)
	return m, nil
}

func (r *Recorder) enqueue(id string) {
	if r.queue != nil {
		r.queue.Enqueue(id, "", false)
	}
}

// toggle starts or stops: what the shortcut and the menu bar do.
func (r *Recorder) toggle() {
	r.mu.Lock()
	running := r.id != ""
	r.mu.Unlock()
	var err error
	if running {
		_, err = r.Stop()
	} else {
		_, err = r.Start()
	}
	if err != nil {
		log.Println("recorder:", err)
	}
}

// shutdown stops a running recording when the app quits and waits for its m4a.
func (r *Recorder) shutdown() {
	r.mu.Lock()
	running := r.id != ""
	r.mu.Unlock()
	if !running {
		return
	}
	if m, err := r.Stop(); err == nil {
		if st, err := r.store(); err == nil {
			finalizeAudio(st, m.ID) // Stop's own conversion would be cut off by the exit
		}
	}
}

// State returns what the page needs on load.
func (r *Recorder) State() RecordingState {
	r.mu.Lock()
	defer r.mu.Unlock()
	return RecordingState{Recording: r.id != "", ID: r.id, Elapsed: r.rec.Elapsed(), Level: r.rec.Level()}
}

// Discard deletes the recording that was just stopped (the undo toast).
// Any other id is refused.
func (r *Recorder) Discard(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if id == "" || id != r.last {
		return errors.New("only the last recording can be discarded")
	}
	st, err := r.store()
	if err != nil {
		return err
	}
	dir, err := st.Path(id)
	if err != nil {
		return err
	}
	r.last = ""
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	_ = LibraryChanged.Broadcast(id) // the page reloads its list
	return nil
}

// finalizeAudio encodes the recording's WAVs as m4a. One track becomes
// audio.m4a. With system audio there are three: mic.m4a and system.m4a, which
// are transcribed and rebuilt from, and audio.m4a, their mix, which is played.
func finalizeAudio(st library.Store, id string) {
	dir, err := st.Path(id)
	if err != nil {
		return
	}
	mic, sys := filepath.Join(dir, wavName), filepath.Join(dir, systemWAVName)
	if _, err := os.Stat(sys); err == nil {
		mix := filepath.Join(dir, "mix.wav")
		defer os.Remove(mix)
		for _, c := range []struct{ wav, m4a string }{{mic, micM4AName}, {sys, systemM4AName}} {
			if err := audio.ToM4A(c.wav, filepath.Join(dir, c.m4a)); err != nil {
				log.Println("m4a:", err)
				return
			}
		}
		if err := audio.MixWAV(mix, mic, sys); err != nil {
			log.Println("mix:", err)
			return
		}
		mic = mix // audio.m4a is encoded from the mix below
	}
	if err := audio.ToM4A(mic, filepath.Join(dir, m4aName)); err != nil {
		log.Println("m4a:", err)
		return
	}
	// The pipeline may have finished first and kept the wav for lack of an m4a.
	if r, err := st.Get(id); err == nil && r.Meta.Status == library.StatusDone {
		pipeline.CleanupWAV(dir)
	}
}

// recoverRecordings finishes recordings the app left in "recording" after a
// crash: the WAV is valid up to the last flush.
func recoverRecordings(st library.Store) {
	metas, err := st.List()
	if err != nil {
		return
	}
	for _, m := range metas {
		dir, _ := st.Path(m.ID)
		if m.Status == library.StatusImporting { // the conversion was cut off
			m.Status, m.Error = library.StatusFailed, locales.ImportInterrupted()
			_ = st.SaveMeta(m)
			continue
		}
		if m.Status != library.StatusRecording {
			// stopped, but the app quit before the m4a was made
			_, noM4A := os.Stat(filepath.Join(dir, m4aName))
			if _, hasWAV := os.Stat(filepath.Join(dir, wavName)); hasWAV == nil && noM4A != nil {
				go finalizeAudio(st, m.ID)
			}
			continue
		}
		fi, err := os.Stat(filepath.Join(dir, wavName))
		if err != nil || fi.Size() <= 44 {
			_ = os.RemoveAll(dir) // nothing was captured
			continue
		}
		m.Duration = float64(fi.Size()-44) / 32000
		m.Status = library.StatusQueued
		m.Error = locales.RecordingRecovered()
		if st.SaveMeta(m) == nil {
			go finalizeAudio(st, m.ID)
		}
	}
}

// CanCaptureSystem reports whether this build can record system audio.
func (r *Recorder) CanCaptureSystem() bool { return systemaudio.Supported }
