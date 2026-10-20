package summarize

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"transflowy/internal/transcribe"
)

// OllamaBase is where Ollama serves its OpenAI-compatible API.
const OllamaBase = "http://localhost:11434/v1"

// Chat calls /chat/completions of OpenRouter or any OpenAI-compatible server.
// Client must be the netguard one: a transcript is sent through it.
type Chat struct {
	Client  *http.Client
	BaseURL string
	Key     func() (string, error) // nil: no Authorization header (Ollama)
	Model   string
}

// Name identifies the model in logs.
func (c Chat) Name() string { return c.Model }

func (c Chat) Complete(ctx context.Context, system, user string) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"model": c.Model,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
	})
	req, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.Key != nil {
		key, err := c.Key()
		if err != nil {
			return "", err
		}
		if key != "" {
			req.Header.Set("Authorization", "Bearer "+key)
		}
	}
	resp, err := c.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if resp.StatusCode/100 != 2 {
		return "", &transcribe.HTTPError{Status: resp.StatusCode, Body: string(data[:min(len(data), 300)])}
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 || out.Choices[0].Message.Content == "" {
		return "", errors.New("empty answer")
	}
	return out.Choices[0].Message.Content, nil
}

// DetectOllama lists the models of a running Ollama, nil when it is not
// running. base is the Ollama root, e.g. http://localhost:11434.
func DetectOllama(ctx context.Context, client *http.Client, base string) []string {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", base+"/api/tags", nil)
	if err != nil {
		return nil
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	var out struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if resp.StatusCode != 200 || json.NewDecoder(resp.Body).Decode(&out) != nil {
		return nil
	}
	names := make([]string, len(out.Models))
	for i, m := range out.Models {
		names[i] = m.Name
	}
	return names
}
