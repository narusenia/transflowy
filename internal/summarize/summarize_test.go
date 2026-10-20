package summarize

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"transflowy/internal/library"
	"transflowy/internal/pipeline"
)

func TestFormat(t *testing.T) {
	tr := library.Transcript{
		Segments: []library.Segment{
			{Start: 5, Speaker: library.UnknownSpeaker, Text: "はじめます"},
			{Start: 4500, Speaker: 1, Text: "予算の件"},
		},
		Speakers: map[string]string{"1": "田中"},
	}
	want := "[00:05] はじめます\n[75:00] 田中: 予算の件\n"
	if got := Format(tr); got != want {
		t.Fatalf("got %q", got)
	}
}

func TestSplitTitle(t *testing.T) {
	title, body := SplitTitle("TITLE: 週次定例\n\n## 概要\n本文")
	if title != "週次定例" || body != "## 概要\n本文" {
		t.Fatalf("%q %q", title, body)
	}
	if title, body := SplitTitle("## 概要\n本文"); title != "" || body != "## 概要\n本文" {
		t.Fatalf("no title line: %q %q", title, body)
	}
}

func TestTemplates(t *testing.T) {
	dir := t.TempDir()
	if n := len(Templates(dir, "ja")); n != 6 {
		t.Fatalf("built-in templates: %d", n)
	}
	os.MkdirAll(filepath.Join(dir, "templates"), 0o755)
	os.WriteFile(filepath.Join(dir, "templates", "mine.md"), []byte("---\nname: 週報/まとめ\ndescription: d\n---\n週報にする\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "templates", "x.md"), []byte("---\nname: 議事録\n---\n上書き\n"), 0o644)
	var mine, minutes *Template
	all := Templates(dir, "ja")
	for i := range all {
		switch all[i].Name {
		case "週報-まとめ":
			mine = &all[i]
		case "議事録":
			minutes = &all[i]
		}
	}
	if mine == nil || mine.Prompt != "週報にする" || mine.Builtin {
		t.Fatalf("user template: %+v", mine)
	}
	if minutes == nil || minutes.Prompt != "上書き" || len(all) != 7 {
		t.Fatalf("override: %+v, %d templates", minutes, len(all))
	}
}

func fakeServer(t *testing.T, answer string, got *map[string]any) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(got)
		json.NewEncoder(w).Encode(map[string]any{"choices": []map[string]any{{"message": map[string]string{"content": answer}}}})
	}))
}

func TestQueueSummarizes(t *testing.T) {
	var sent map[string]any
	srv := fakeServer(t, "TITLE: 予算会議\n## 概要\n予算が決まった [00:05]", &sent)
	defer srv.Close()

	st := library.Store{Root: t.TempDir()}
	m, _ := st.Create("", time.Now())
	st.SaveTranscript(m.ID, library.Transcript{Segments: []library.Segment{{Start: 5, Speaker: library.UnknownSpeaker, Text: "予算を決める"}}})
	st.UpdateMeta(m.ID, func(m *library.Meta) { m.Status = library.StatusDone })

	ready := false
	q := New(Config{
		Store:     func() (library.Store, error) { return st, nil },
		Templates: func() []Template { return Templates(st.Root, "ja") },
		Default:   func() string { return "議事録" },
		Chat: func() (Chat, error) {
			if !ready {
				return Chat{}, pipeline.NotReadyError{Msg: "no model"}
			}
			return Chat{Client: srv.Client(), BaseURL: srv.URL, Model: "m"}, nil
		},
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go q.Run(ctx)
	wait := func(want library.Status) library.Meta {
		for i := 0; i < 200; i++ {
			r, _ := st.Get(m.ID)
			if r.Meta.Status == want && (want != library.StatusSummarizing || r.Meta.Error != "") {
				return r.Meta
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatalf("never reached %s", want)
		return library.Meta{}
	}

	q.Enqueue(m.ID, "")
	if got := wait(library.StatusSummarizing); got.Error != "no model" {
		t.Fatalf("waiting: %+v", got)
	}
	ready = true
	q.Enqueue(m.ID, "")
	got := wait(library.StatusDone)
	if got.Title != "予算会議" || got.Error != "" {
		t.Fatalf("meta: %+v", got)
	}
	text, err := st.Summary(m.ID, "議事録")
	if err != nil || !strings.HasPrefix(text, "## 概要") {
		t.Fatalf("summary: %q %v", text, err)
	}
	msgs := sent["messages"].([]any)
	if user := msgs[1].(map[string]any)["content"]; user != "[00:05] 予算を決める\n" {
		t.Fatalf("sent %q", user)
	}
}

func TestTemplatesEnglish(t *testing.T) {
	all := Templates(t.TempDir(), "en-US")
	if len(all) != 6 || all[0].Name != "Minutes" || all[5].Key != MemoKey || all[5].Name != "Memo" {
		t.Fatalf("english built-ins: %+v", all)
	}
}
