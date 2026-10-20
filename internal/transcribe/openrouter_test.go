package transcribe

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSentences(t *testing.T) {
	sp := func(n int) *int { return &n }
	words := []orWord{
		{"お", 0, 0.1, sp(0)}, {"は", 0.1, 0.2, sp(0)}, {"よ", 0.2, 0.3, sp(0)}, {"う", 0.3, 0.4, sp(0)}, {"。", 0.4, 0.5, sp(0)},
		{"API", 1, 1.2, sp(1)}, {"は", 1.2, 1.3, sp(1)}, // same speaker, no mark: Japanese and Latin join without a space
		{"Thanks.", 2, 2.4, sp(1)}, {"The", 2.5, 2.6, sp(1)}, {"staging", 2.6, 3, sp(1)}, {"works", 3, 3.4, sp(1)}, {"fine", 3.4, 3.6, sp(0)},
	}
	got := sentences(words)
	want := []string{"おはよう。", "APIはThanks.", "The staging works", "fine"}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for i, w := range want {
		if got[i].Text != w || got[i].Original != w {
			t.Errorf("line %d = %q, want %q", i, got[i].Text, w)
		}
	}
	if got[0].Start != 0 || got[0].End != 0.5 || got[0].Speaker != 0 || got[1].Speaker != 1 || got[3].Speaker != 0 {
		t.Errorf("times/speakers: %+v", got[:2])
	}
}

func TestOpenRouterRequest(t *testing.T) {
	var body map[string]any
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		json.NewDecoder(r.Body).Decode(&body)
		w.Write([]byte(`{"language":"ja","segments":[{"start":0,"end":2,"text":" こんにちは ","speaker":1}]}`))
	}))
	defer srv.Close()

	o := OpenRouter{Client: srv.Client(), Key: func() (string, error) { return "k", nil }, BaseURL: srv.URL}
	res, err := o.Transcribe(context.Background(), Request{WAV: []byte("RIFF"), Phrases: []string{"田中"}, Diarize: true})
	if err != nil {
		t.Fatal(err)
	}
	if auth != "Bearer k" || body["model"] != DefaultCloudModel || body["language"] != nil {
		t.Errorf("request: %v %v", auth, body)
	}
	az := body["provider"].(map[string]any)["options"].(map[string]any)["azure"].(map[string]any)
	if az["diarization"].(map[string]any)["enabled"] != true || az["phraseList"].(map[string]any)["phrases"].([]any)[0] != "田中" {
		t.Errorf("azure options: %v", az)
	}
	if res.Language != "ja" || len(res.Segments) != 1 || res.Segments[0].Text != "こんにちは" || res.Segments[0].Speaker != 1 {
		t.Errorf("result: %+v", res)
	}
}

func TestErrorKinds(t *testing.T) {
	if Retryable(&HTTPError{Status: 401}) || !Retryable(&HTTPError{Status: 429}) || !Retryable(&HTTPError{Status: 503}) {
		t.Error("retryable statuses")
	}
}
