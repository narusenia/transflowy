// Package audio writes WAV files and converts audio with the system's tools.
package audio

import (
	"encoding/binary"
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
