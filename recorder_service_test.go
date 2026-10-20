package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

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
