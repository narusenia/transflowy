package main

import (
	"bytes"
	"image/png"
	"testing"
)

func TestTrayIconVariants(t *testing.T) {
	seen := map[string]bool{}
	for _, rec := range []bool{false, true} {
		for _, locked := range []bool{false, true} {
			b := trayIcon(rec, locked)
			img, err := png.Decode(bytes.NewReader(b))
			if err != nil || img.Bounds().Dx() != 32 {
				t.Fatalf("rec=%v locked=%v: %v", rec, locked, err)
			}
			seen[string(b)] = true
		}
	}
	if len(seen) != 4 { // idle, recording, and a padlock on each
		t.Fatalf("%d distinct icons, want 4", len(seen))
	}
}
