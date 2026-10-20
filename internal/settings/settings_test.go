package settings

import (
	"errors"
	"path/filepath"
	"testing"
)

var errNone = errors.New("none")

type fakeKeys map[string][]byte

func (f fakeKeys) Set(k string, v []byte) error { f[k] = v; return nil }
func (f fakeKeys) Delete(k string) error {
	if _, ok := f[k]; !ok {
		return errNone
	}
	delete(f, k)
	return nil
}
func (f fakeKeys) Get(k string) ([]byte, error) {
	if v, ok := f[k]; ok {
		return v, nil
	}
	return nil, errNone
}

func TestSettingsAndKeys(t *testing.T) {
	dir := t.TempDir()
	s := &Service{Path: filepath.Join(dir, "cfg", "settings.json"), Home: "/home/u", Keys: fakeKeys{}, ErrKeyNotFound: errNone}

	got, err := s.Get() // nothing saved: defaults
	if err != nil || got.LibraryDir != "/home/u/Documents/Transflowy" || got.Transcribe.Kind != KindLocal {
		t.Fatalf("defaults: %+v %v", got, err)
	}

	got.LocalOnly = true
	got.Summarize.Model = "m"
	if err := s.Set(got); err != nil {
		t.Fatal(err)
	}
	back, _ := s.Get()
	if !back.LocalOnly || back.Summarize.Model != "m" || back.Shortcut != "Alt+Cmd+R" {
		t.Fatalf("roundtrip: %+v", back)
	}

	if ok, _ := s.HasKey("openrouter"); ok {
		t.Fatal("key should be absent")
	}
	s.SetKey("openrouter", "secret")
	if ok, _ := s.HasKey("openrouter"); !ok {
		t.Fatal("key should be present")
	}
	if v, _ := ReadKey(s, "openrouter"); v != "secret" {
		t.Fatal("ReadKey")
	}
	if err := s.SetKey("openrouter", ""); err != nil { // empty clears
		t.Fatal(err)
	}
	if err := s.DeleteKey("openrouter"); err != nil { // deleting a missing key is fine
		t.Fatal(err)
	}
}
