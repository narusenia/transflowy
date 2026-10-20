package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"transflowy/internal/audio"
	"transflowy/internal/library"
	"transflowy/internal/settings"
)

func TestDiscardOnlyLast(t *testing.T) {
	root := t.TempDir()
	cfg := &settings.Service{Path: filepath.Join(t.TempDir(), "s.json"), Home: t.TempDir()}
	if err := cfg.Set(settings.Settings{LibraryDir: root}); err != nil {
		t.Fatal(err)
	}
	r := &Recorder{cfg: cfg}
	st := library.Store{Root: root}
	a, _ := st.Create("a", time.Now())
	b, _ := st.Create("b", time.Now())
	r.last = b.ID

	if err := r.Discard(a.ID); err == nil { // not the last one: refused
		t.Fatal("discarded a recording that was not the last")
	}
	if err := r.Discard(b.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, b.ID)); !os.IsNotExist(err) {
		t.Fatalf("folder still there: %v", err)
	}
	if metas, _ := st.List(); len(metas) != 1 || metas[0].ID != a.ID {
		t.Fatalf("list: %+v", metas)
	}
}

func TestFinalizeAudioMakesThreeFilesForTwoTracks(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("afconvert")
	}
	st := library.Store{Root: t.TempDir()}
	m, _ := st.Create("x", time.Now())
	dir, _ := st.Path(m.ID)
	for _, name := range []string{wavName, systemWAVName} {
		w, _ := audio.CreateWAV(filepath.Join(dir, name), 16000)
		w.Write(make([]byte, 16000*2*2)) // 2 s of silence
		w.Close()
	}
	finalizeAudio(st, m.ID)
	for _, f := range []string{m4aName, micM4AName, systemM4AName} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Errorf("%s: %v", f, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "mix.wav")); err == nil {
		t.Error("mix.wav was left behind")
	}
}
