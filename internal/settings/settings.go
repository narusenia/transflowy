// Package settings keeps the app's settings in settings.json and API keys in
// the OS keychain. A key's value never leaves this package towards the page.
package settings

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

// Provider kinds.
const (
	KindLocal        = "local"         // bundled whisper-server (transcribe only)
	KindOpenRouter   = "openrouter"    // https://openrouter.ai/api/v1
	KindOpenAICompat = "openai-compat" // any OpenAI-compatible base URL
)

// Provider picks the engine for one job (transcribe or summarize).
type Provider struct {
	Kind    string `json:"kind"`
	Model   string `json:"model"`
	BaseURL string `json:"baseUrl,omitempty"` // KindOpenAICompat only
}

// Settings is settings.json.
type Settings struct {
	LibraryDir string   `json:"libraryDir"`
	Transcribe Provider `json:"transcribe"`
	Summarize  Provider `json:"summarize"`
	// Template is the summary template an automatic summary uses.
	Template string `json:"template"`
	// LocalOnly forbids every request to a cloud provider.
	LocalOnly bool `json:"localOnly"`
	// InputDevice is the microphone's name; "" is the system default.
	InputDevice string `json:"inputDevice"`
	Shortcut    string `json:"shortcut"`
	Onboarded   bool   `json:"onboarded"`
}

// Defaults returns the settings a fresh install starts with.
func Defaults(home string) Settings {
	return Settings{
		LibraryDir: filepath.Join(home, "Documents", "Transflowy"),
		Transcribe: Provider{Kind: KindLocal},
		Summarize:  Provider{Kind: KindOpenRouter}, // Model stays empty: the user must pick one
		Template:   "汎用メモ",
		Shortcut:   "Alt+Cmd+R",
	}
}

// KeyStore is where API keys live; mygo.Secrets in the app.
type KeyStore interface {
	Set(key string, value []byte) error
	Get(key string) ([]byte, error)
	Delete(key string) error
}

// Service is bound to the page.
type Service struct {
	Path string // settings.json
	Home string
	Keys KeyStore
	// ErrKeyNotFound is the KeyStore's "no such key" error.
	ErrKeyNotFound error
	// OnChange is called after settings or a key changed.
	OnChange func()

	mu sync.Mutex
}

// Get returns the saved settings, or the defaults when none are saved yet.
func (s *Service) Get() (Settings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.load()
}

func (s *Service) load() (Settings, error) {
	cur := Defaults(s.Home)
	b, err := os.ReadFile(s.Path)
	if errors.Is(err, os.ErrNotExist) {
		return cur, nil
	}
	if err != nil {
		return cur, err
	}
	// Unmarshal over the defaults so a field added later keeps its default.
	return cur, json.Unmarshal(b, &cur)
}

// Set replaces the settings.
func (s *Service) Set(v Settings) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o755); err != nil {
		return err
	}
	tmp := s.Path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, s.Path); err != nil {
		return err
	}
	s.changed()
	return nil
}

func (s *Service) changed() {
	if s.OnChange != nil {
		go s.OnChange() // not under the caller's lock or call
	}
}

// SetKey stores an API key for a provider kind ("openrouter", "openai-compat").
func (s *Service) SetKey(kind, value string) error {
	if value == "" {
		return s.DeleteKey(kind)
	}
	if err := s.Keys.Set(kind, []byte(value)); err != nil {
		return err
	}
	s.changed()
	return nil
}

// DeleteKey removes a stored key. Removing a missing key is not an error.
func (s *Service) DeleteKey(kind string) error {
	if err := s.Keys.Delete(kind); err != nil && !errors.Is(err, s.ErrKeyNotFound) {
		return err
	}
	s.changed()
	return nil
}

// HasKey reports whether a key is stored. The page never gets the value.
func (s *Service) HasKey(kind string) (bool, error) {
	_, err := s.Keys.Get(kind)
	if errors.Is(err, s.ErrKeyNotFound) {
		return false, nil
	}
	return err == nil, err
}

// ReadKey returns the stored key for a provider kind, "" when none is stored.
// It is a function, not a method, so mygo does not bind it to the page.
func ReadKey(s *Service, kind string) (string, error) {
	b, err := s.Keys.Get(kind)
	if errors.Is(err, s.ErrKeyNotFound) {
		return "", nil
	}
	return string(b), err
}
