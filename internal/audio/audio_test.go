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
	// afconvert pads its WAV with a 4 KB chunk: samples must still be found.
	padded := filepath.Join(dir, "padded.wav")
	if err := ToWAV16k(wav, padded); err != nil {
		t.Fatal(err)
	}
	if st, _ := os.Stat(padded); st.Size() <= int64(44+len(pcm)) {
		t.Log("afconvert wrote no padding here")
	}
	want, _ := ReadPCM(wav, 0, 1<<20)
	got, err := ReadPCM(padded, 0, 1<<20)
	if err != nil || len(got) != len(want) || got[1234] != want[1234] || got[15999] != want[15999] {
		t.Fatalf("padded wav: %d samples, err %v", len(got), err)
	}
	if n, _ := PCMSamples(padded); n != 16000 {
		t.Fatalf("PCMSamples = %d", n)
	}
	if tail, _ := ReadPCM(padded, 15000, 5000); len(tail) != 1000 { // clamped at the end
		t.Fatalf("tail %d", len(tail))
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

func TestPeaks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.wav")
	w, _ := CreateWAV(path, 16000)
	pcm := make([]int16, 1000) // quiet, then one loud sample in the last tenth
	pcm[950] = -16384
	b := make([]byte, 0, 2000)
	for _, s := range pcm {
		b = append(b, byte(s), byte(uint16(s)>>8))
	}
	w.Write(b)
	w.Close()
	p, err := Peaks(path, 10)
	if err != nil || len(p) != 10 || p[9] != 0.5 || p[0] != 0 {
		t.Fatalf("%v %v", p, err)
	}
}

func TestMixWAV(t *testing.T) {
	dir := t.TempDir()
	mk := func(name string, vals []int16) string {
		p := filepath.Join(dir, name)
		w, _ := CreateWAV(p, 16000)
		w.Write(WAVBytes(vals, 16000)[44:])
		w.Close()
		return p
	}
	a := mk("a.wav", []int16{100, 30000, -30000})
	b := mk("b.wav", []int16{1, 30000})
	out := filepath.Join(dir, "m.wav")
	if err := MixWAV(out, a, b); err != nil {
		t.Fatal(err)
	}
	pcm, _ := ReadPCM(out, 0, 10)
	if len(pcm) != 3 || pcm[0] != 101 || pcm[1] != 32767 || pcm[2] != -30000 {
		t.Fatalf("%v", pcm)
	}
}
