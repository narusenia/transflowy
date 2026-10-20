// Package recorder captures the microphone as 16 kHz mono PCM into a WAV file.
package recorder

import (
	"errors"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gen2brain/malgo"

	"transflowy/internal/audio"
)

const (
	sampleRate    = 16000
	flushInterval = 2 * time.Second // a crash loses at most this much audio
)

// Recorder records one session at a time.
type Recorder struct {
	mu      sync.Mutex
	ctx     *malgo.AllocatedContext
	dev     *malgo.Device
	wav     *audio.WAV
	buf     []byte // captured, not yet written
	writeEr error  // first write error, reported by Stop
	stop    chan struct{}
	done    chan struct{}
	samples atomic.Int64
	level   atomic.Uint32 // float32 bits: peak of the last callback, 0..1
}

// ErrBusy is returned by Start while a recording is running.
var ErrBusy = errors.New("already recording")

// Devices lists the names of the input devices.
func Devices() ([]string, error) {
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = ctx.Uninit(); ctx.Free() }()
	infos, err := ctx.Devices(malgo.Capture)
	names := make([]string, len(infos))
	for i := range infos {
		names[i] = infos[i].Name()
	}
	return names, err
}

// Start opens the input device called device and records into the WAV file at
// path. An empty or unknown name (an unplugged headset) uses the default
// device, so a recording is always possible. macOS asks for microphone
// permission the first time.
func (r *Recorder) Start(path, device string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.dev != nil {
		return ErrBusy
	}
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		return err
	}
	w, err := audio.CreateWAV(path, sampleRate)
	if err != nil {
		_ = ctx.Uninit()
		ctx.Free()
		return err
	}
	cfg := malgo.DefaultDeviceConfig(malgo.Capture)
	cfg.Capture.Format = malgo.FormatS16
	cfg.Capture.Channels = 1
	cfg.SampleRate = sampleRate
	if device != "" {
		if infos, err := ctx.Devices(malgo.Capture); err == nil {
			for i := range infos {
				if infos[i].Name() == device {
					// ponytail: Pointer copies the id into C memory malgo gives no way to free; 64 bytes per recording.
					cfg.Capture.DeviceID = infos[i].ID.Pointer()
					break
				}
			}
		}
	}
	r.buf, r.writeEr = nil, nil
	r.samples.Store(0)
	r.level.Store(0)
	dev, err := malgo.InitDevice(ctx.Context, cfg, malgo.DeviceCallbacks{Data: r.onData})
	if err == nil {
		err = dev.Start()
		if err != nil {
			dev.Uninit()
		}
	}
	if err != nil {
		w.Close()
		_ = ctx.Uninit()
		ctx.Free()
		return err
	}
	r.ctx, r.dev, r.wav = ctx, dev, w
	r.stop, r.done = make(chan struct{}), make(chan struct{})
	go r.flushLoop(r.stop, r.done)
	return nil
}

// onData runs on the audio thread: copy and return.
func (r *Recorder) onData(_, in []byte, _ uint32) {
	var peak int
	for i := 0; i+1 < len(in); i += 2 {
		v := int(int16(uint16(in[i]) | uint16(in[i+1])<<8))
		if v < 0 {
			v = -v
		}
		peak = max(peak, v)
	}
	r.level.Store(math.Float32bits(float32(peak) / 32768))
	r.samples.Add(int64(len(in) / 2))
	r.mu.Lock()
	r.buf = append(r.buf, in...)
	r.mu.Unlock()
}

func (r *Recorder) flushLoop(stop, done chan struct{}) {
	defer close(done)
	t := time.NewTicker(flushInterval)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			r.flush()
		case <-stop:
			return
		}
	}
}

func (r *Recorder) flush() {
	r.mu.Lock()
	b := r.buf
	r.buf = nil
	w := r.wav
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
	if r.dev == nil {
		r.mu.Unlock()
		return 0, errors.New("not recording")
	}
	dev, ctx, stop, done := r.dev, r.ctx, r.stop, r.done
	r.mu.Unlock()

	_ = dev.Stop()
	dev.Uninit()
	close(stop)
	<-done
	r.flush()

	r.mu.Lock()
	defer r.mu.Unlock()
	err := r.writeEr
	if cerr := r.wav.Close(); err == nil {
		err = cerr
	}
	_ = ctx.Uninit()
	ctx.Free()
	r.dev, r.ctx, r.wav = nil, nil, nil
	return float64(r.samples.Load()) / sampleRate, err
}

// Elapsed is the length recorded so far in seconds.
func (r *Recorder) Elapsed() float64 { return float64(r.samples.Load()) / sampleRate }

// Level is the peak of the last audio callback, 0..1.
func (r *Recorder) Level() float32 { return math.Float32frombits(r.level.Load()) }

// Recording reports whether a session is running.
func (r *Recorder) Recording() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.dev != nil
}
