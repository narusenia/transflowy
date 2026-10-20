package systemaudio

import (
	"path/filepath"
	"testing"
	"time"

	"transflowy/internal/audio"
)

// 48 kHz stereo in, 16 kHz mono out: a third of the samples, channels averaged.
func TestOnDataResamplesToMono16k(t *testing.T) {
	r := &Recorder{stop: func() {}, step: 3}
	w, _ := audio.CreateWAV(filepath.Join(t.TempDir(), "s.wav"), sampleRate)
	r.wav = w
	r.started = timeNowForTest()
	frames := make([]float32, 4800*2)
	for i := 0; i < len(frames); i += 2 {
		frames[i], frames[i+1] = 0.5, 0.1 // mean 0.3
	}
	r.onData(frames, 2)
	if n := r.samples.Load(); n < 1590 || n > 1610 {
		t.Fatalf("samples: %d", n)
	}
	if got := int16(uint16(r.buf[len(r.buf)-2]) | uint16(r.buf[len(r.buf)-1])<<8); got < 9700 || got > 9900 { // 0.3 * 32767
		t.Fatalf("last sample %d", got)
	}
}

func timeNowForTest() time.Time { return time.Now() }
