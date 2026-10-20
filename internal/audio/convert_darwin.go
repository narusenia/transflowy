package audio

import (
	"fmt"
	"os/exec"
)

func afconvert(args ...string) error {
	if out, err := exec.Command("/usr/bin/afconvert", args...).CombinedOutput(); err != nil {
		return fmt.Errorf("afconvert: %w: %s", err, out)
	}
	return nil
}

// ToWAV16k decodes any file afconvert reads (m4a, mp3, wav, aac, caf, and the
// audio of mp4/mov) into 16 kHz mono 16-bit WAV, the engines' input.
func ToWAV16k(src, dst string) error {
	return afconvert(src, "-f", "WAVE", "-d", "LEI16@16000", "-c", "1", dst)
}

// ToM4A encodes a 16 kHz mono WAV as AAC at 32 kbps (about 14 MB per hour).
// -c must not be given and 64 kbps is refused at 16 kHz mono.
func ToM4A(src, dst string) error {
	return afconvert(src, "-f", "m4af", "-d", "aac", "-b", "32000", dst)
}
