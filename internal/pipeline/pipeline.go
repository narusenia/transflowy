// Package pipeline turns recordings into transcripts, one chunk at a time. It
// works on a recording that is still being captured as well as on a finished
// one, saves after every chunk so a restart continues, and waits out a lost
// connection.
package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"

	"transflowy/internal/audio"
	"transflowy/internal/library"
	"transflowy/internal/netguard"
	"transflowy/internal/transcribe"
	"transflowy/locales"
)

const (
	wavName = "recording.wav"
	m4aName = "audio.m4a"
	// Samples read per pass: a long recording is planned in windows, not held whole.
	readLimit = 20 * 60 * transcribe.SampleRate
	livePoll  = time.Second
	maxTries  = 5

	// An engine that finds one language per chunk drops speech in another one:
	// when a chunk's text stops this long before its audio does and speech
	// follows, the rest is sent again as a chunk of its own.
	droppedTail = 3 * transcribe.SampleRate
	minCovered  = transcribe.SampleRate // a result shorter than this makes no progress worth repeating
)

// Job is what the engine factory returns for one recording.
type Job struct {
	Engine  transcribe.Engine
	Params  transcribe.Params
	Diarize bool
}

// NotReadyError means the engine cannot work yet: no API key, no model, or
// local-only on with a cloud engine chosen. The recording keeps waiting and is
// processed when the settings change; it is not a failure.
type NotReadyError struct{ Msg string }

func (e NotReadyError) Error() string { return e.Msg }

// Config connects the queue to the app. Sleep is for tests.
type Config struct {
	Store   func() (library.Store, error)
	Engine  func(kind string) (Job, error) // kind "" means the setting
	Phrases func() []string
	Changed func(id string)
	// Transcribed is called once a recording's transcript is complete.
	Transcribed func(id string)
	Sleep       func(ctx context.Context, d time.Duration) bool
}

type request struct {
	id      string
	kind    string
	restart bool // throw away the transcript and start over
}

// Queue runs one recording at a time.
type Queue struct {
	cfg     Config
	mu      sync.Mutex
	pending []request
	cancel  map[string]context.CancelFunc // the running job's
	running string
	wake    chan struct{}
}

// Sleep waits d; false when ctx ended first.
func Sleep(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-ctx.Done():
		return false
	}
}

func New(cfg Config) *Queue {
	if cfg.Sleep == nil {
		cfg.Sleep = Sleep
	}
	return &Queue{cfg: cfg, cancel: map[string]context.CancelFunc{}, wake: make(chan struct{}, 1)}
}

// Enqueue adds a recording. One already waiting is replaced; one already
// running is stopped and queued again, which is how "reprocess with another
// engine" takes over. restart discards what was transcribed so far.
func (q *Queue) Enqueue(id, kind string, restart bool) {
	q.mu.Lock()
	for i, r := range q.pending {
		if r.id == id {
			q.pending = append(q.pending[:i], q.pending[i+1:]...)
			break
		}
	}
	q.pending = append(q.pending, request{id, kind, restart})
	if c := q.cancel[id]; c != nil {
		c()
	}
	q.mu.Unlock()
	select {
	case q.wake <- struct{}{}:
	default:
	}
}

// Run processes the queue until ctx ends.
func (q *Queue) Run(ctx context.Context) {
	for {
		q.mu.Lock()
		var req request
		have := len(q.pending) > 0
		if have {
			req = q.pending[0]
			q.pending = q.pending[1:]
		}
		q.mu.Unlock()
		if !have {
			select {
			case <-q.wake:
				continue
			case <-ctx.Done():
				return
			}
		}
		jctx, cancel := context.WithCancel(ctx)
		q.mu.Lock()
		q.cancel[req.id] = cancel
		q.mu.Unlock()
		q.process(jctx, req)
		cancel()
		q.mu.Lock()
		delete(q.cancel, req.id)
		q.mu.Unlock()
		if ctx.Err() != nil {
			return
		}
	}
}

func (q *Queue) changed(id string) {
	if q.cfg.Changed != nil {
		q.cfg.Changed(id)
	}
}

func (q *Queue) process(ctx context.Context, req request) {
	st, err := q.cfg.Store()
	if err != nil {
		log.Println("pipeline:", err)
		return
	}
	id := req.id
	dir, err := st.Path(id)
	if err != nil {
		return
	}
	if req.restart {
		_ = st.UpdateTranscript(id, func(t *library.Transcript) { t.Segments = []library.Segment{} })
		_, _ = st.UpdateMeta(id, func(m *library.Meta) { m.ProcessedUntil, m.Detected, m.Error = 0, "", "" })
	}
	rec, err := st.Get(id)
	if err != nil {
		return
	}
	job, err := q.cfg.Engine(req.kind)
	var nr NotReadyError
	if errors.As(err, &nr) {
		if rec.Meta.Status != library.StatusRecording {
			_, _ = st.UpdateMeta(id, func(m *library.Meta) { m.Status, m.Error = library.StatusQueued, nr.Msg })
			q.changed(id)
		}
		return
	}
	if err != nil {
		q.fail(st, id, rec.Meta.Status == library.StatusRecording, err)
		return
	}

	wav := filepath.Join(dir, wavName)
	if _, err := os.Stat(wav); err != nil { // processed before: rebuild it from the m4a
		if rec.Meta.Status == library.StatusRecording {
			return
		}
		if err := audio.ToWAV16k(filepath.Join(dir, m4aName), wav); err != nil {
			q.fail(st, id, false, err)
			return
		}
	}

	done := int(rec.Meta.ProcessedUntil * transcribe.SampleRate)
	for ctx.Err() == nil {
		// Read the status before the audio: a recording that is no longer
		// "recording" has all its audio on disk.
		cur, err := st.Get(id)
		if err != nil {
			return
		}
		live := cur.Meta.Status == library.StatusRecording
		if !live && cur.Meta.Status != library.StatusProcessing {
			_, _ = st.UpdateMeta(id, func(m *library.Meta) { m.Status = library.StatusProcessing })
			q.changed(id)
		}
		region, err := audio.ReadPCM(wav, done, readLimit)
		if err != nil {
			q.fail(st, id, live, err)
			return
		}
		final := !live && len(region) < readLimit
		spans, _ := transcribe.Plan(region, 0, final, job.Params)
		resend := false // the chunk lost its tail: plan again from where its text ends
		for _, sp := range spans {
			if ctx.Err() != nil {
				return
			}
			end := done + sp.End
			if sp.Speech {
				covered, err := q.transcribeSpan(ctx, st, id, job, cur.Meta.Language, region[sp.Start:sp.End], done+sp.Start)
				if err != nil {
					if ctx.Err() == nil {
						q.fail(st, id, live, err)
					}
					return
				}
				if tail := sp.End - sp.Start - covered; covered >= minCovered && tail > droppedTail && transcribe.HasSpeech(region[sp.Start+covered:sp.End]) {
					end, resend = done+sp.Start+covered, true
				}
			}
			t := float64(end) / transcribe.SampleRate
			_, _ = st.UpdateMeta(id, func(m *library.Meta) { m.ProcessedUntil = t })
			q.changed(id)
			if resend {
				done = end
				break
			}
		}
		if resend {
			continue
		}
		if n := len(spans); n > 0 {
			done += spans[n-1].End
		}
		switch {
		case live:
			if !q.cfg.Sleep(ctx, livePoll) {
				return
			}
		case len(spans) == 0 && final:
			q.finish(st, id, dir)
			return
		}
	}
}

// transcribeSpan sends one chunk and appends its lines to the transcript.
func (q *Queue) transcribeSpan(ctx context.Context, st library.Store, id string, job Job, lang string, pcm []int16, startSample int) (covered int, err error) {
	req := transcribe.Request{WAV: audio.WAVBytes(pcm, transcribe.SampleRate), Language: lang, Phrases: q.cfg.Phrases(), Diarize: job.Diarize}
	res, err := q.send(ctx, st, id, job.Engine, req)
	if err != nil {
		return 0, err
	}
	offset := float64(startSample) / transcribe.SampleRate
	for i := range res.Segments {
		res.Segments[i].Start += offset
		res.Segments[i].End += offset
	}
	if err := st.UpdateTranscript(id, func(t *library.Transcript) { t.Segments = append(t.Segments, res.Segments...) }); err != nil {
		return 0, err
	}
	if n := len(res.Segments); n > 0 {
		covered = int((res.Segments[n-1].End - offset) * transcribe.SampleRate)
	}
	_, err = st.UpdateMeta(id, func(m *library.Meta) {
		m.Engine, m.Error = job.Engine.Name(), ""
		if res.Language != "" {
			m.Detected = res.Language
		}
	})
	return covered, err
}

// send retries a chunk. A lost connection is waited out for as long as it
// lasts; other temporary failures get a few tries; a bad key or a refused
// file fails at once.
func (q *Queue) send(ctx context.Context, st library.Store, id string, e transcribe.Engine, req transcribe.Request) (transcribe.Result, error) {
	backoff, tries := 2*time.Second, 0
	note := func(text string) {
		_, _ = st.UpdateMeta(id, func(m *library.Meta) { m.Error = text })
		q.changed(id)
	}
	for {
		res, err := e.Transcribe(ctx, req)
		if err == nil {
			return res, nil
		}
		if ctx.Err() != nil || !transcribe.Retryable(err) {
			return transcribe.Result{}, err
		}
		if transcribe.Offline(err) {
			note(locales.StatusOffline())
		} else if tries++; tries >= maxTries {
			return transcribe.Result{}, err
		} else {
			note(locales.StatusRetrying())
		}
		if !q.cfg.Sleep(ctx, backoff) {
			return transcribe.Result{}, ctx.Err()
		}
		backoff = min(backoff*2, 30*time.Second)
	}
}

// fail records why a recording could not be processed. For a recording that
// is still being captured it only gives up on live processing: the recorder
// queues it again when it stops, and then a failure is shown.
func (q *Queue) fail(st library.Store, id string, live bool, err error) {
	log.Println("pipeline:", id, err)
	if live {
		return
	}
	_, _ = st.UpdateMeta(id, func(m *library.Meta) {
		m.Status, m.Error = library.StatusFailed, describe(err)
	})
	q.changed(id)
}

func describe(err error) string {
	var he *transcribe.HTTPError
	switch {
	case errors.Is(err, netguard.ErrLocalOnly):
		return err.Error()
	case errors.As(err, &he) && (he.Status == 401 || he.Status == 403):
		return locales.ErrBadKey()
	}
	return locales.ErrProcessing(err.Error())
}

func (q *Queue) finish(st library.Store, id, dir string) {
	_, _ = st.UpdateMeta(id, func(m *library.Meta) { m.Status, m.Error = library.StatusDone, "" })
	savePeaks(dir)
	CleanupWAV(dir)
	q.changed(id)
	if q.cfg.Transcribed != nil {
		q.cfg.Transcribed(id)
	}
}

// peakCount is how many bars the player's waveform has.
const peakCount = 2000

// savePeaks writes peaks.json from recording.wav, so the page draws the
// waveform without decoding a long audio file. Call it before CleanupWAV.
func savePeaks(dir string) {
	peaks, err := audio.Peaks(filepath.Join(dir, wavName), peakCount)
	if err != nil || len(peaks) == 0 {
		return
	}
	for i, p := range peaks { // 3 decimals are plenty for a picture
		peaks[i] = float32(math.Round(float64(p)*1000) / 1000)
	}
	b, _ := json.Marshal(peaks)
	tmp := filepath.Join(dir, "peaks.json.tmp")
	if os.WriteFile(tmp, b, 0o644) == nil {
		_ = os.Rename(tmp, filepath.Join(dir, "peaks.json"))
	}
}

// CleanupWAV removes recording.wav once the m4a exists and the transcript is
// done: the m4a is what is kept (the wav is rebuilt from it to reprocess).
func CleanupWAV(dir string) {
	if _, err := os.Stat(filepath.Join(dir, m4aName)); err != nil {
		return
	}
	_ = os.Remove(filepath.Join(dir, wavName))
}
