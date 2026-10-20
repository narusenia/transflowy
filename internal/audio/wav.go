// Package audio writes WAV files and converts audio with the system's tools.
package audio

import (
	"encoding/binary"
	"errors"
	"io"
	"os"
)

// WAV is a 16-bit mono PCM file that is valid after every Write: the header
// sizes are rewritten each time, so a crash loses at most the last chunk.
type WAV struct {
	f *os.File
	n uint32 // bytes of PCM written
}

// CreateWAV creates the file and writes its header.
func CreateWAV(path string, sampleRate int) (*WAV, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	h := make([]byte, 44)
	copy(h[0:], "RIFF")
	copy(h[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(h[16:], 16)
	binary.LittleEndian.PutUint16(h[20:], 1) // PCM
	binary.LittleEndian.PutUint16(h[22:], 1) // mono
	binary.LittleEndian.PutUint32(h[24:], uint32(sampleRate))
	binary.LittleEndian.PutUint32(h[28:], uint32(sampleRate)*2)
	binary.LittleEndian.PutUint16(h[32:], 2)
	binary.LittleEndian.PutUint16(h[34:], 16)
	copy(h[36:], "data")
	binary.LittleEndian.PutUint32(h[4:], 36)
	if _, err := f.Write(h); err != nil {
		f.Close()
		return nil, err
	}
	return &WAV{f: f}, nil
}

// Write appends little-endian 16-bit samples and fixes the header sizes.
func (w *WAV) Write(pcm []byte) error {
	if _, err := w.f.WriteAt(pcm, 44+int64(w.n)); err != nil {
		return err
	}
	w.n += uint32(len(pcm))
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], 36+w.n)
	if _, err := w.f.WriteAt(b[:], 4); err != nil {
		return err
	}
	binary.LittleEndian.PutUint32(b[:], w.n)
	_, err := w.f.WriteAt(b[:], 40)
	return err
}

// Close closes the file.
func (w *WAV) Close() error { return w.f.Close() }

// WAVBytes returns a complete 16-bit mono WAV file for pcm.
func WAVBytes(pcm []int16, sampleRate int) []byte {
	n := len(pcm) * 2
	b := make([]byte, 44+n)
	copy(b[0:], "RIFF")
	binary.LittleEndian.PutUint32(b[4:], uint32(36+n))
	copy(b[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(b[16:], 16)
	binary.LittleEndian.PutUint16(b[20:], 1)
	binary.LittleEndian.PutUint16(b[22:], 1)
	binary.LittleEndian.PutUint32(b[24:], uint32(sampleRate))
	binary.LittleEndian.PutUint32(b[28:], uint32(sampleRate)*2)
	binary.LittleEndian.PutUint16(b[32:], 2)
	binary.LittleEndian.PutUint16(b[34:], 16)
	copy(b[36:], "data")
	binary.LittleEndian.PutUint32(b[40:], uint32(n))
	for i, v := range pcm {
		binary.LittleEndian.PutUint16(b[44+i*2:], uint16(v))
	}
	return b
}

// dataChunk finds the samples in a WAV: after the header chunks, which
// afconvert pads with a 4 KB FLLR chunk, so the data does not start at 44. The
// length is clamped to the file: a recording in progress is only as long as
// what was written.
func dataChunk(f *os.File) (offset, length int64, err error) {
	fi, err := f.Stat()
	if err != nil {
		return 0, 0, err
	}
	pos := int64(12) // after "RIFF<size>WAVE"
	var h [8]byte
	for pos+8 <= fi.Size() {
		if _, err := f.ReadAt(h[:], pos); err != nil {
			return 0, 0, err
		}
		size := int64(binary.LittleEndian.Uint32(h[4:]))
		if string(h[:4]) == "data" {
			return pos + 8, min(size, fi.Size()-pos-8), nil
		}
		pos += 8 + size + size&1 // chunks are padded to an even size
	}
	return 0, 0, errors.New("wav: no data chunk")
}

// PCMSamples returns how many samples a 16-bit mono WAV holds.
func PCMSamples(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	_, n, err := dataChunk(f)
	return int(n / 2), err
}

// ReadPCM reads up to limit samples of a 16-bit mono WAV, starting at sample
// from. It returns fewer than limit samples at the end of the file.
func ReadPCM(path string, from, limit int) ([]int16, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	off, length, err := dataChunk(f)
	if err != nil {
		return nil, err
	}
	want := min(int64(limit)*2, length-int64(from)*2)
	if want <= 0 {
		return []int16{}, nil
	}
	buf := make([]byte, want)
	n, err := f.ReadAt(buf, off+int64(from)*2)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	out := make([]int16, n/2)
	for i := range out {
		out[i] = int16(binary.LittleEndian.Uint16(buf[i*2:]))
	}
	return out, nil
}

// Peaks returns n values in 0..1: the loudest sample of each of n equal
// slices of a 16-bit mono WAV. It reads in windows, so a long recording is
// never held whole.
func Peaks(path string, n int) ([]float32, error) {
	total, err := PCMSamples(path)
	if err != nil || total == 0 || n <= 0 {
		return nil, err
	}
	n = min(n, total)
	out := make([]float32, n)
	const window = 60 * 16000
	for from := 0; from < total; from += window {
		pcm, err := ReadPCM(path, from, window)
		if err != nil {
			return nil, err
		}
		for i, s := range pcm {
			v := float32(s) / 32768
			if v < 0 {
				v = -v
			}
			if k := int(int64(from+i) * int64(n) / int64(total)); v > out[k] {
				out[k] = v
			}
		}
	}
	return out, nil
}
