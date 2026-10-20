// Package systemaudio records what the computer plays (the other side of a
// call) into a 16 kHz mono WAV file, like the microphone recorder does.
package systemaudio

import (
	"errors"
	"math"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"transflowy/internal/audio"
)

const sampleRate = 16000

// ErrBusy is returned by Start while a recording is running.
var ErrBusy = errors.New("already recording")

// Recorder records one session at a time.
type Recorder struct {
	mu      sync.Mutex
	stop    func()
	wav     *audio.WAV
	buf     []byte
	writeEr error
	samples atomic.Int64
	level   atomic.Uint32 // float32 bits: peak of the last callback, 0..1

	started time.Time

	// resampler state: position in input samples of the next output sample
	step, pos, prev float64
}

// Start taps the system's audio output and records it into the WAV file at
// path. The first call asks for the "System Audio Recording" permission.
func (r *Recorder) Start(path string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.stop != nil {
		return ErrBusy
	}
	w, err := audio.CreateWAV(path, sampleRate)
	if err != nil {
		return err
	}
	r.wav, r.buf, r.writeEr = w, nil, nil
	r.samples.Store(0)
	r.level.Store(0)
	r.pos, r.prev = 0, 0
	r.started = time.Now()
	stop, rate, _, err := startTap(r.onData)
	if err != nil {
		_ = w.Close()
		_ = os.Remove(path) // an empty system track must not make the recording a two-track one
		r.wav = nil
		return err
	}
	r.step = rate / sampleRate
	r.stop = stop
	return nil
}

// onData runs on the audio thread: mix to mono, resample, buffer.
func (r *Recorder) onData(frames []float32, channels int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.stop == nil && r.wav == nil {
		return
	}
	// The tap delivers nothing while the Mac is silent: fill the gap, so this
	// track stays level with the microphone's.
	r.padTo(r.wallSamples() - int64(float64(len(frames)/channels)/r.step))
	var peak float32
	for i := 0; i+channels <= len(frames); i += channels {
		var m float32
		for c := 0; c < channels; c++ {
			m += frames[i+c]
		}
		m /= float32(channels)
		if a := float32(math.Abs(float64(m))); a > peak {
			peak = a
		}
		// linear interpolation between the previous and this input sample
		for r.pos < 1 {
			v := r.prev + (float64(m)-r.prev)*r.pos
			r.buf = append(r.buf, pcm(v)...)
			r.samples.Add(1)
			r.pos += r.step
		}
		r.pos--
		r.prev = float64(m)
	}
	r.level.Store(math.Float32bits(peak))
}

func (r *Recorder) wallSamples() int64 { return int64(time.Since(r.started).Seconds() * sampleRate) }

// padTo appends silence until the track holds n samples. Callers hold r.mu.
func (r *Recorder) padTo(n int64) {
	if gap := n - r.samples.Load(); gap > sampleRate/10 { // under 100 ms is just jitter
		r.buf = append(r.buf, make([]byte, gap*2)...)
		r.samples.Add(gap)
		r.pos = 0
	}
}

func pcm(v float64) []byte {
	s := int16(math.Max(-1, math.Min(1, v)) * 32767)
	return []byte{byte(s), byte(uint16(s) >> 8)}
}

// Flush writes what was captured so far to the file. Call it every few seconds.
func (r *Recorder) Flush() {
	r.mu.Lock()
	if r.stop != nil {
		r.padTo(r.wallSamples())
	}
	b, w := r.buf, r.wav
	r.buf = nil
	r.mu.Unlock()
	if len(b) == 0 || w == nil {
		return
	}
	if err := w.Write(b); err != nil {
		r.mu.Lock()
		if r.writeEr == nil {
			r.writeEr = err
		}
		r.mu.Unlock()
	}
}

// Stop ends the recording and returns its length in seconds.
func (r *Recorder) Stop() (float64, error) {
	r.mu.Lock()
	stop := r.stop
	r.mu.Unlock()
	if stop == nil {
		return 0, errors.New("not recording")
	}
	stop()
	r.Flush()
	r.mu.Lock()
	defer r.mu.Unlock()
	r.padTo(r.wallSamples())
	if len(r.buf) > 0 {
		_ = r.wav.Write(r.buf)
		r.buf = nil
	}
	err := r.writeEr
	if cerr := r.wav.Close(); err == nil {
		err = cerr
	}
	r.stop, r.wav = nil, nil
	return float64(r.samples.Load()) / sampleRate, err
}

// Level is the peak of the last audio callback, 0..1.
func (r *Recorder) Level() float32 { return math.Float32frombits(r.level.Load()) }

// Recording reports whether a session is running.
func (r *Recorder) Recording() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.stop != nil
}
