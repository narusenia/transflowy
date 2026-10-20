package transcribe

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"strings"

	"transflowy/internal/library"
)

// Whisper posts a WAV as multipart to a whisper-server (/inference) or to any
// OpenAI-compatible /audio/transcriptions.
type Whisper struct {
	Client *http.Client
	URL    string                 // the full endpoint
	Model  string                 // OpenAI-compatible servers only
	Key    func() (string, error) // nil: no Authorization header (the local server)
	Local  bool                   // whisper-server: language "auto" instead of omitted
}

func (w Whisper) Name() string {
	if w.Local {
		return "local:whisper-large-v3-turbo"
	}
	return "openai-compat:" + w.Model
}

type whisperResponse struct {
	Language string `json:"language"`
	Text     string `json:"text"`
	Segments []struct {
		Start float64 `json:"start"`
		End   float64 `json:"end"`
		Text  string  `json:"text"`
	} `json:"segments"`
}

func (w Whisper) Transcribe(ctx context.Context, r Request) (Result, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	f, _ := mw.CreateFormFile("file", "chunk.wav")
	f.Write(r.WAV)
	field := func(k, v string) { _ = mw.WriteField(k, v) }
	field("response_format", "verbose_json")
	field("temperature", "0")
	switch {
	case r.Language != "":
		field("language", r.Language)
	case w.Local:
		field("language", "auto")
	}
	if w.Model != "" {
		field("model", w.Model)
	}
	if len(r.Phrases) > 0 {
		field("prompt", strings.Join(r.Phrases, ", "))
	}
	mw.Close()

	req, err := http.NewRequestWithContext(ctx, "POST", w.URL, &buf)
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if w.Key != nil {
		key, err := w.Key()
		if err != nil {
			return Result{}, err
		}
		if key != "" {
			req.Header.Set("Authorization", "Bearer "+key)
		}
	}
	resp, err := w.Client.Do(req)
	if err != nil {
		return Result{}, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if resp.StatusCode/100 != 2 {
		return Result{}, &HTTPError{resp.StatusCode, truncate(string(data), 300)}
	}
	var out whisperResponse
	if err := json.Unmarshal(data, &out); err != nil {
		return Result{}, err
	}
	res := Result{Language: out.Language}
	for _, s := range out.Segments {
		if strings.TrimSpace(s.Text) != "" {
			res.Segments = append(res.Segments, segment(s.Start, s.End, -1, s.Text))
		}
	}
	if len(res.Segments) == 0 && strings.TrimSpace(out.Text) != "" { // no timestamps: one line for the chunk
		res.Segments = []library.Segment{segment(0, 0, -1, out.Text)}
	}
	return res, nil
}
