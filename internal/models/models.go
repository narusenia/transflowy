// Package models downloads the local engine's model file.
package models

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Whisper is the local transcription model: large-v3-turbo, 5-bit quantized.
var Whisper = Model{
	File: "ggml-large-v3-turbo-q5_0.bin",
	URL:  "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-large-v3-turbo-q5_0.bin",
}

// Model is a file to fetch into the models directory.
type Model struct{ File, URL string }

// Path returns where the model lives under dir.
func (m Model) Path(dir string) string { return filepath.Join(dir, m.File) }

// Present reports whether the model is fully downloaded.
func (m Model) Present(dir string) bool {
	fi, err := os.Stat(m.Path(dir))
	return err == nil && fi.Size() > 0
}

// Download fetches the model into dir, continuing a partial file from an
// earlier attempt, and checks the SHA-256 the server names (Hugging Face sends
// it as X-Linked-ETag). The file appears under its final name only when whole.
// This is a plain client on purpose: it sends nothing of the user's, so the
// local-only guard does not apply.
func (m Model) Download(ctx context.Context, dir string, progress func(done, total int64)) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	part := m.Path(dir) + ".part"
	var have int64
	if fi, err := os.Stat(part); err == nil {
		have = fi.Size()
	}
	req, err := http.NewRequestWithContext(ctx, "GET", m.URL, nil)
	if err != nil {
		return err
	}
	if have > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", have))
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	flags := os.O_CREATE | os.O_WRONLY
	switch resp.StatusCode {
	case http.StatusPartialContent:
		flags |= os.O_APPEND
	case http.StatusOK: // the server ignored Range: start over
		flags |= os.O_TRUNC
		have = 0
	default:
		return fmt.Errorf("download: HTTP %d", resp.StatusCode)
	}
	total := have + resp.ContentLength
	f, err := os.OpenFile(part, flags, 0o644)
	if err != nil {
		return err
	}
	done := have
	buf := make([]byte, 256<<10)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, err := f.Write(buf[:n]); err != nil {
				f.Close()
				return err
			}
			done += int64(n)
			if progress != nil {
				progress(done, total)
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			f.Close()
			return rerr
		}
	}
	if err := f.Close(); err != nil {
		return err
	}
	if want := strings.Trim(resp.Header.Get("X-Linked-Etag"), `"`); len(want) == 64 {
		if got, err := sum(part); err != nil || got != want {
			_ = os.Remove(part) // a corrupt file is not resumed
			return fmt.Errorf("download: checksum mismatch")
		}
	}
	return os.Rename(part, m.Path(dir))
}

func sum(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
