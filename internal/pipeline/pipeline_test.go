package pipeline

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"net"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"transflowy/internal/audio"
	"transflowy/internal/library"
	"transflowy/internal/transcribe"
	"transflowy/locales"
)

type fake struct {
	mu    sync.Mutex
	calls int
	errs  []error // returned in order before succeeding
}

func (f *fake) Name() string { return "fake" }
func (f *fake) Transcribe(_ context.Context, r transcribe.Request) (transcribe.Result, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if len(f.errs) > 0 {
		err := f.errs[0]
		f.errs = f.errs[1:]
		return transcribe.Result{}, err
	}
	return transcribe.Result{Language: "ja", Segments: []library.Segment{{Start: 0, End: 1, Speaker: -1, Text: "x", Original: "x"}}}, nil
}

// tone: speech-like audio, silence: a pause.
func writeAudio(t *testing.T, path string, parts ...float64) {
	t.Helper()
	w, err := audio.CreateWAV(path, transcribe.SampleRate)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	for i, sec := range parts {
		pcm := make([]byte, int(sec*transcribe.SampleRate)*2)
		if i%2 == 0 { // even parts are tone, odd are silence
			for j := 0; j < len(pcm)/2; j++ {
				binary.LittleEndian.PutUint16(pcm[j*2:], uint16(int16(8000*math.Sin(2*math.Pi*300*float64(j)/transcribe.SampleRate))))
			}
		}
		if err := w.Write(pcm); err != nil {
			t.Fatal(err)
		}
	}
}

type env struct {
	st  library.Store
	id  string
	q   *Queue
	eng *fake
}

func setup(t *testing.T, status library.Status, parts ...float64) *env {
	t.Helper()
	st := library.Store{Root: t.TempDir()}
	m, _ := st.Create("t", time.Now())
	dir, _ := st.Path(m.ID)
	writeAudio(t, filepath.Join(dir, wavName), parts...)
	st.UpdateMeta(m.ID, func(m *library.Meta) { m.Status = status })
	e := &env{st: st, id: m.ID, eng: &fake{}}
	e.q = New(Config{
		Store:   func() (library.Store, error) { return st, nil },
		Engine:  func(string) (Job, error) { return Job{Engine: e.eng, Params: transcribe.LocalParams}, nil },
		Phrases: func() []string { return nil },
		Sleep:   func(ctx context.Context, _ time.Duration) bool { return ctx.Err() == nil },
	})
	return e
}

func (e *env) run(t *testing.T) library.Recording {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go e.q.Run(ctx)
	e.q.Enqueue(e.id, "", false)
	for i := 0; i < 500; i++ {
		r, _ := e.st.Get(e.id)
		if r.Meta.Status == library.StatusDone || r.Meta.Status == library.StatusFailed {
			return r
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timeout")
	return library.Recording{}
}

func TestProcessesWholeRecording(t *testing.T) {
	// 5 s tone, 1 s pause, 6 s tone, 0.6 s pause, 20 s tone: three local chunks.
	e := setup(t, library.StatusQueued, 5, 1, 6, 0.6, 20)
	r := e.run(t)
	if r.Meta.Status != library.StatusDone || len(r.Transcript.Segments) != 3 || r.Meta.Detected != "ja" || r.Meta.Engine != "fake" {
		t.Fatalf("%+v segs=%d", r.Meta, len(r.Transcript.Segments))
	}
	if s := r.Transcript.Segments; !(s[0].Start == 0 && s[1].Start > 12 && s[2].Start > s[1].Start) { // offsets added
		t.Errorf("segment starts: %v %v %v", s[0].Start, s[1].Start, s[2].Start)
	}
	if r.Meta.ProcessedUntil < 32 {
		t.Errorf("processed until %v", r.Meta.ProcessedUntil)
	}
}

func TestResumeSkipsDoneAudio(t *testing.T) {
	e := setup(t, library.StatusQueued, 5, 1, 6, 0.6, 20)
	e.st.UpdateMeta(e.id, func(m *library.Meta) { m.ProcessedUntil = 12.3 })
	r := e.run(t)
	if e.eng.calls != 2 || r.Transcript.Segments[0].Start < 12 {
		t.Fatalf("calls=%d first=%v", e.eng.calls, r.Transcript.Segments)
	}
}

func TestRetries(t *testing.T) {
	e := setup(t, library.StatusQueued, 5)
	e.eng.errs = []error{&transcribe.HTTPError{Status: 503}, &transcribe.HTTPError{Status: 429}}
	if r := e.run(t); r.Meta.Status != library.StatusDone || e.eng.calls != 3 {
		t.Fatalf("%v calls=%d", r.Meta.Status, e.eng.calls)
	}

	// Being offline is waited out and never counted as a failed try.
	e = setup(t, library.StatusQueued, 5)
	off := &net.OpError{Op: "dial", Err: fmt.Errorf("no route")}
	e.eng.errs = []error{off, off, off, off, off, off, off, off}
	if r := e.run(t); r.Meta.Status != library.StatusDone || e.eng.calls != 9 {
		t.Fatalf("offline: %v calls=%d", r.Meta.Status, e.eng.calls)
	}

	// Too many temporary failures give up; a bad key gives up at once.
	e = setup(t, library.StatusQueued, 5)
	e.eng.errs = []error{&transcribe.HTTPError{Status: 500}, &transcribe.HTTPError{Status: 500}, &transcribe.HTTPError{Status: 500}, &transcribe.HTTPError{Status: 500}, &transcribe.HTTPError{Status: 500}}
	if r := e.run(t); r.Meta.Status != library.StatusFailed || e.eng.calls != maxTries {
		t.Fatalf("gave up: %v calls=%d", r.Meta.Status, e.eng.calls)
	}
	e = setup(t, library.StatusQueued, 5)
	e.eng.errs = []error{&transcribe.HTTPError{Status: 401}}
	if r := e.run(t); r.Meta.Status != library.StatusFailed || e.eng.calls != 1 || r.Meta.Error != locales.ErrBadKey() {
		t.Fatalf("bad key: %v calls=%d %q", r.Meta.Status, e.eng.calls, r.Meta.Error)
	}
}

func TestWorksWhileStillRecording(t *testing.T) {
	e := setup(t, library.StatusRecording, 5, 1, 6, 0.6, 20)
	polls := 0
	e.q.cfg.Sleep = func(ctx context.Context, _ time.Duration) bool {
		if polls++; polls == 2 { // the recorder stops: what it left is complete
			e.st.UpdateMeta(e.id, func(m *library.Meta) { m.Status = library.StatusQueued })
		}
		return ctx.Err() == nil
	}
	r := e.run(t)
	if r.Meta.Status != library.StatusDone || len(r.Transcript.Segments) != 3 || polls < 2 {
		t.Fatalf("%v segs=%d polls=%d", r.Meta.Status, len(r.Transcript.Segments), polls)
	}
}

func TestNotReadyWaitsInsteadOfFailing(t *testing.T) {
	e := setup(t, library.StatusQueued, 5)
	e.q.cfg.Engine = func(string) (Job, error) { return Job{}, NotReadyError{"no key"} }
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go e.q.Run(ctx)
	e.q.Enqueue(e.id, "", false)
	time.Sleep(200 * time.Millisecond)
	r, _ := e.st.Get(e.id)
	if r.Meta.Status != library.StatusQueued || r.Meta.Error != "no key" {
		t.Fatalf("%v %q", r.Meta.Status, r.Meta.Error)
	}
}
