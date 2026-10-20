package transcribe

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

// Uses a real whisper-server and model when both exist (the model path comes
// from TRANSFLOWY_TEST_MODEL); otherwise it skips.
func TestServerWithRealModel(t *testing.T) {
	model := os.Getenv("TRANSFLOWY_TEST_MODEL")
	wav := os.Getenv("TRANSFLOWY_TEST_WAV")
	if _, err := FindBinary(); err != nil || model == "" || wav == "" {
		t.Skip("set TRANSFLOWY_TEST_MODEL and TRANSFLOWY_TEST_WAV, and install whisper-server")
	}
	s := &Server{Model: model}
	defer s.Stop()
	url, err := s.URL(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(wav)
	w := Whisper{Client: http.DefaultClient, URL: url, Local: true}
	res, err := w.Transcribe(context.Background(), Request{WAV: data, Phrases: []string{"ダッシュボード"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Segments) == 0 || res.Segments[0].Text == "" {
		t.Fatalf("no text: %+v", res)
	}
	t.Logf("%s: %s", filepath.Base(wav), res.Segments[0].Text)

	s.Stop() // the next call starts it again
	if _, err := s.URL(context.Background()); err != nil {
		t.Fatal(err)
	}
	_ = io.Discard
}

func TestServerMissingModel(t *testing.T) {
	s := &Server{Model: filepath.Join(t.TempDir(), "none.bin")}
	if _, err := FindBinary(); err != nil {
		t.Skip("no whisper-server")
	}
	if _, err := s.URL(context.Background()); err == nil {
		t.Fatal("expected an error for a missing model")
	}
}
