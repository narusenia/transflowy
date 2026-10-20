package models

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDownloadResumesAndVerifies(t *testing.T) {
	data := make([]byte, 1<<20)
	for i := range data {
		data[i] = byte(i * 7)
	}
	h := sha256.Sum256(data)
	etag := hex.EncodeToString(h[:])
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Linked-Etag", `"`+etag+`"`)
		http.ServeContent(w, r, "m.bin", timeZero, bytesReader(data)) // honours Range
	}))
	defer srv.Close()

	dir := t.TempDir()
	m := Model{File: "m.bin", URL: srv.URL}
	os.WriteFile(m.Path(dir)+".part", data[:300000], 0o644) // an interrupted attempt
	var last int64
	if err := m.Download(context.Background(), dir, func(done, total int64) { last = done; _ = total }); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(m.Path(dir))
	if len(got) != len(data) || last != int64(len(data)) || !m.Present(dir) {
		t.Fatalf("size %d, progress %d", len(got), last)
	}
	if _, err := os.Stat(filepath.Join(dir, "m.bin.part")); !os.IsNotExist(err) {
		t.Error("part file left behind")
	}

	// A part file that is not a prefix of the real file fails the checksum and is removed.
	bad := Model{File: "bad.bin", URL: srv.URL}
	os.WriteFile(bad.Path(dir)+".part", []byte("junk"), 0o644)
	if err := bad.Download(context.Background(), dir, nil); err == nil || bad.Present(dir) {
		t.Fatalf("corrupt download accepted: %v", err)
	}
}

var timeZero = time.Time{}

func bytesReader(b []byte) *bytes.Reader { return bytes.NewReader(b) }
