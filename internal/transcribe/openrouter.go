package transcribe

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"transflowy/internal/library"
)

// DefaultCloudModel is the cloud model when the settings name none.
const DefaultCloudModel = "microsoft/mai-transcribe-2"

// OpenRouter calls OpenRouter's /audio/transcriptions with a base64 WAV.
type OpenRouter struct {
	Client  *http.Client
	Key     func() (string, error)
	Model   string
	BaseURL string // default https://openrouter.ai/api/v1
}

func (o OpenRouter) Name() string { return "openrouter:" + o.model() }

func (o OpenRouter) model() string {
	if o.Model == "" {
		return DefaultCloudModel
	}
	return o.Model
}

type orWord struct {
	Word    string  `json:"word"`
	Start   float64 `json:"start"`
	End     float64 `json:"end"`
	Speaker *int    `json:"speaker"`
}

type orResponse struct {
	Language string `json:"language"`
	Segments []struct {
		Start   float64 `json:"start"`
		End     float64 `json:"end"`
		Text    string  `json:"text"`
		Speaker *int    `json:"speaker"`
	} `json:"segments"`
	Words []orWord `json:"words"`
}

func (o OpenRouter) Transcribe(ctx context.Context, r Request) (Result, error) {
	key, err := o.Key()
	if err != nil {
		return Result{}, err
	}
	body := map[string]any{
		"model":                   o.model(),
		"input_audio":             map[string]string{"data": base64.StdEncoding.EncodeToString(r.WAV), "format": "wav"},
		"response_format":         "verbose_json",
		"timestamp_granularities": []string{"segment", "word"},
	}
	if r.Language != "" {
		body["language"] = r.Language
	}
	if strings.HasPrefix(o.model(), "microsoft/mai-transcribe") { // served by Azure: its options
		az := map[string]any{}
		if r.Diarize {
			az["diarization"] = map[string]any{"enabled": true}
		}
		if len(r.Phrases) > 0 {
			az["phraseList"] = map[string]any{"phrases": r.Phrases}
		}
		if len(az) > 0 {
			body["provider"] = map[string]any{"options": map[string]any{"azure": az}}
		}
	}
	b, _ := json.Marshal(body)
	base := o.BaseURL
	if base == "" {
		base = "https://openrouter.ai/api/v1"
	}
	req, err := http.NewRequestWithContext(ctx, "POST", base+"/audio/transcriptions", bytes.NewReader(b))
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := o.Client.Do(req)
	if err != nil {
		return Result{}, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if resp.StatusCode/100 != 2 {
		return Result{}, &HTTPError{resp.StatusCode, truncate(string(data), 300)}
	}
	var out orResponse
	if err := json.Unmarshal(data, &out); err != nil {
		return Result{}, err
	}
	res := Result{Language: out.Language}
	if len(out.Words) > 0 {
		res.Segments = sentences(out.Words)
	} else {
		for _, s := range out.Segments {
			res.Segments = append(res.Segments, segment(s.Start, s.End, speakerOrUnknown(s.Speaker), s.Text))
		}
	}
	return res, nil
}

// sentences makes transcript lines from words: one line per sentence, and a
// new line when the speaker changes. Without diarization the engine returns
// segments of minutes, which are useless as lines to click. Japanese comes as
// single characters and English as words without spaces, so a space is put
// between two Latin words.
func sentences(words []orWord) []library.Segment {
	var out []library.Segment
	var b strings.Builder
	var start, end float64
	speaker, open := library.UnknownSpeaker, false
	flush := func() {
		if open {
			out = append(out, segment(start, end, speaker, b.String()))
			b.Reset()
			open = false
		}
	}
	for _, w := range words {
		sp := speakerOrUnknown(w.Speaker)
		if open && sp != speaker {
			flush()
		}
		if !open {
			start, speaker, open = w.Start, sp, true
		}
		if b.Len() > 0 && latin(lastRune(b.String())) && latin([]rune(w.Word)[0]) {
			b.WriteByte(' ')
		}
		b.WriteString(w.Word)
		end = w.End
		if strings.ContainsRune(".!?。！？", lastRune(w.Word)) {
			flush()
		}
	}
	flush()
	return out
}

func lastRune(s string) rune {
	r := []rune(s)
	return r[len(r)-1]
}

// latin reports an ASCII letter, digit or sentence mark: what English words are made of.
func latin(r rune) bool {
	return r < 128 && (r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || strings.ContainsRune(".,!?;:'\")%", r))
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
