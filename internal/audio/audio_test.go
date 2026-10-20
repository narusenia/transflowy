package audio

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestWAVAndConvert(t *testing.T) {
	dir := t.TempDir()
	wav := filepath.Join(dir, "a.wav")
	w, err := CreateWAV(wav, 16000)
	if err != nil {
		t.Fatal(err)
	}
	pcm := make([]byte, 16000*2) // 1 s of 440 Hz
	for i := 0; i < 16000; i++ {
		binary.LittleEndian.PutUint16(pcm[i*2:], uint16(int16(8000*math.Sin(2*math.Pi*440*float64(i)/16000))))
	}
	// Two writes; the file must be a valid WAV before Close.
	if err := w.Write(pcm[:10000]); err != nil {
		t.Fatal(err)
	}
	if err := w.Write(pcm[10000:]); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(wav)
	if got := binary.LittleEndian.Uint32(b[40:]); got != uint32(len(pcm)) || len(b) != 44+len(pcm) {
		t.Fatalf("data size %d, file %d", got, len(b))
	}
	if got := binary.LittleEndian.Uint32(b[4:]); got != uint32(36+len(pcm)) {
		t.Fatalf("riff size %d", got)
	}
	w.Close()

	if runtime.GOOS != "darwin" {
		t.Skip("conversion is macOS only")
	}
	m4a := filepath.Join(dir, "a.m4a")
	back := filepath.Join(dir, "back.wav")
	if err := ToM4A(wav, m4a); err != nil {
		t.Fatal(err)
	}
	if err := ToWAV16k(m4a, back); err != nil {
		t.Fatal(err)
	}
	if st, err := os.Stat(back); err != nil || st.Size() < 30000 {
		t.Fatalf("decoded wav: %v %v", st, err)
	}
}
