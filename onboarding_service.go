package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"transflowy/internal/audio"
	"transflowy/internal/pipeline"
	"transflowy/internal/recorder"
	"transflowy/internal/settings"
	"transflowy/internal/transcribe"
	"transflowy/locales"
)

const trialLength = 10 * time.Second

// Onboarding is callable from the first-run screen.
type Onboarding struct {
	cfg     *settings.Service
	engines *Engines
	app     *Recorder // the app's own recorder: onboarding never records while it runs
	rec     recorder.Recorder
}

// MicResult says what the permission check heard.
type MicResult struct {
	// Heard is false when the input was completely silent. That can be a quiet
	// room, but it is also what macOS delivers while permission is denied.
	Heard bool `json:"heard"`
}

func (o *Onboarding) record(length time.Duration) (path string, peak float32, err error) {
	if o.app.rec.Recording() {
		return "", 0, errors.New("already recording")
	}
	dir, err := os.MkdirTemp("", "transflowy-onboarding-")
	if err != nil {
		return "", 0, err
	}
	cur, _ := o.cfg.Get()
	path = filepath.Join(dir, "trial.wav")
	peak, err = o.rec.Record(path, cur.InputDevice, length)
	return path, peak, err
}

// CheckMic opens the microphone for a moment, which makes macOS ask for permission.
func (o *Onboarding) CheckMic() (MicResult, error) {
	path, peak, err := o.record(700 * time.Millisecond)
	if path != "" {
		_ = os.RemoveAll(filepath.Dir(path))
	}
	return MicResult{Heard: peak > 0}, err
}

// Trial records ten seconds and transcribes them with the engine in the
// settings, to show the result at once. The audio is not kept.
func (o *Onboarding) Trial() (string, error) {
	job, err := o.engines.Build("")
	var nr pipeline.NotReadyError
	if errors.As(err, &nr) {
		return "", nr
	}
	if err != nil {
		return "", err
	}
	path, _, err := o.record(trialLength)
	if path != "" {
		defer os.RemoveAll(filepath.Dir(path))
	}
	if err != nil {
		return "", err
	}
	pcm, err := audio.ReadPCM(path, 0, 1<<30)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	res, err := job.Engine.Transcribe(ctx, transcribe.Request{WAV: audio.WAVBytes(pcm, transcribe.SampleRate), Phrases: phrases(o.cfg)(), Diarize: job.Diarize})
	if err != nil {
		return "", errors.New(locales.ErrProcessing(err.Error()))
	}
	var b strings.Builder
	for _, s := range res.Segments {
		b.WriteString(s.Text + "\n")
	}
	return strings.TrimSpace(b.String()), nil
}
