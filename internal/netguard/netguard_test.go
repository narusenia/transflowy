package netguard

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	defer srv.Close() // 127.0.0.1

	on := true
	c := Client(func() bool { return on })
	if _, err := c.Get("https://openrouter.ai/"); !errors.Is(err, ErrLocalOnly) {
		t.Fatalf("cloud while local-only: %v", err)
	}
	if _, err := c.Get("http://192.168.0.5:8080/"); !errors.Is(err, ErrLocalOnly) {
		t.Fatalf("LAN while local-only: %v", err)
	}
	if resp, err := c.Get(srv.URL); err != nil { // this machine is fine
		t.Fatal(err)
	} else {
		resp.Body.Close()
	}
	on = false // the setting is read on every request
	if _, err := c.Get("http://127.0.0.1:1/"); errors.Is(err, ErrLocalOnly) {
		t.Fatal("blocked with local-only off")
	}
}
