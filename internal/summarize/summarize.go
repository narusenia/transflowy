package summarize

import (
	"context"
	"errors"
	"log"
	"strings"
	"sync"
	"time"

	"transflowy/internal/export"
	"transflowy/internal/library"
	"transflowy/internal/netguard"
	"transflowy/internal/pipeline"
	"transflowy/internal/transcribe"
	"transflowy/locales"
)

const (
	maxTries       = 5
	requestTimeout = 10 * time.Minute
	titlePrefix    = "TITLE:"
)

// preamble comes before every template's own instructions.
const preamble = `あなたは文字起こしを要約するアシスタント。入力は「[分:秒] 発言」形式の文字起こし。
- 回答の 1 行目は「` + titlePrefix + ` 」に続けて 30 字以内のタイトルだけを書く。2 行目以降が要約本文（Markdown）
- 要約は文字起こしと同じ言語で書く
- 重要な事柄には、根拠となる発言の時刻を [12:34] の形式で行末に添える（例: 予算は来月確定する [12:34]）。時刻は入力に現れたものだけを使う
- 文字起こしにない内容は書かない。誤認識らしい語は文脈から補ってよいが、断定しない

`

// Format is the transcript as the model reads it: "[12:34] 話者1: text".
func Format(t library.Transcript) string { return export.Lines(t, true) }

// SplitTitle separates the title line from the summary body. An answer
// without one is all body.
func SplitTitle(answer string) (title, body string) {
	answer = strings.TrimSpace(answer)
	first, rest, _ := strings.Cut(answer, "\n")
	if t, ok := strings.CutPrefix(first, titlePrefix); ok {
		return strings.TrimSpace(t), strings.TrimSpace(rest)
	}
	return "", answer
}

// Config connects the queue to the app. Sleep is for tests.
type Config struct {
	Store     func() (library.Store, error)
	Templates func() []Template
	Default   func() string // the template an automatic summary uses
	// Chat returns pipeline.NotReadyError when no model is chosen, a key is
	// missing or local-only forbids the cloud: the summary then waits.
	Chat    func() (Chat, error)
	Changed func(id string)
	// Finished is called when a summary was written.
	Finished func(id string)
	Sleep    func(ctx context.Context, d time.Duration) bool
}

type request struct{ id, template string }

// Queue writes summaries one at a time.
type Queue struct {
	cfg     Config
	mu      sync.Mutex
	pending []request
	wake    chan struct{}
}

func New(cfg Config) *Queue {
	if cfg.Sleep == nil {
		cfg.Sleep = pipeline.Sleep
	}
	return &Queue{cfg: cfg, wake: make(chan struct{}, 1)}
}

// Enqueue asks for a summary of a recording with a template ("" is the
// default one). The recording shows as waiting for its summary at once.
func (q *Queue) Enqueue(id, template string) {
	if st, err := q.cfg.Store(); err == nil {
		q.status(st, id, library.StatusSummarizing, "")
	}
	q.mu.Lock()
	r := request{id, template}
	dup := false
	for _, p := range q.pending {
		dup = dup || p == r
	}
	if !dup {
		q.pending = append(q.pending, r)
	}
	q.mu.Unlock()
	select {
	case q.wake <- struct{}{}:
	default:
	}
}

// Run processes the queue until ctx ends.
func (q *Queue) Run(ctx context.Context) {
	for {
		q.mu.Lock()
		var req request
		have := len(q.pending) > 0
		if have {
			req, q.pending = q.pending[0], q.pending[1:]
		}
		q.mu.Unlock()
		if !have {
			select {
			case <-q.wake:
				continue
			case <-ctx.Done():
				return
			}
		}
		q.process(ctx, req)
		if ctx.Err() != nil {
			return
		}
	}
}

func (q *Queue) changed(id string) {
	if q.cfg.Changed != nil {
		q.cfg.Changed(id)
	}
}

// status moves a recording between done and summarizing. Any other status
// belongs to the transcript pipeline (a reprocess may have started): left alone.
func (q *Queue) status(st library.Store, id string, s library.Status, errText string) {
	_, _ = st.UpdateMeta(id, func(m *library.Meta) {
		if m.Status == library.StatusDone || m.Status == library.StatusSummarizing {
			m.Status, m.Error = s, errText
		}
	})
	q.changed(id)
}

func (q *Queue) process(ctx context.Context, req request) {
	st, err := q.cfg.Store()
	if err != nil {
		log.Println("summarize:", err)
		return
	}
	rec, err := st.Get(req.id)
	if err != nil {
		return
	}
	name := req.template
	if name == "" {
		name = q.cfg.Default()
	}
	all := q.cfg.Templates()
	tpl := find(all, func(t Template) bool { return t.Name == name })
	if tpl == nil && req.template == "" { // the chosen default is from another language
		tpl = find(all, func(t Template) bool { return t.Key == MemoKey })
	}
	if tpl == nil {
		q.status(st, req.id, library.StatusDone, locales.ErrNoTemplate(name))
		return
	}
	if len(rec.Transcript.Segments) == 0 { // nothing was said: nothing to ask a model
		q.status(st, req.id, library.StatusDone, "")
		return
	}
	chat, err := q.cfg.Chat()
	var nr pipeline.NotReadyError
	if errors.As(err, &nr) {
		q.status(st, req.id, library.StatusSummarizing, nr.Msg) // waits for a settings change
		return
	}
	if err != nil {
		q.status(st, req.id, library.StatusDone, describe(err))
		return
	}
	q.status(st, req.id, library.StatusSummarizing, "")

	answer, err := q.complete(ctx, st, req.id, chat, preamble+tpl.Prompt, Format(rec.Transcript))
	if ctx.Err() != nil {
		return
	}
	if err != nil {
		log.Println("summarize:", req.id, err)
		q.status(st, req.id, library.StatusDone, describe(err))
		return
	}
	title, body := SplitTitle(answer)
	if err := st.SaveSummary(req.id, tpl.Name, body+"\n"); err != nil {
		q.status(st, req.id, library.StatusDone, describe(err))
		return
	}
	_, _ = st.UpdateMeta(req.id, func(m *library.Meta) {
		if m.Title == "" { // never replace a title the user or an import gave
			m.Title = title
		}
	})
	q.status(st, req.id, library.StatusDone, "")
	if q.cfg.Finished != nil {
		q.cfg.Finished(req.id)
	}
}

// complete retries like the transcript pipeline: a lost connection is waited
// out, other temporary failures get a few tries, a bad key fails at once.
func (q *Queue) complete(ctx context.Context, st library.Store, id string, c Chat, system, user string) (string, error) {
	backoff, tries := 2*time.Second, 0
	for {
		rctx, cancel := context.WithTimeout(ctx, requestTimeout)
		out, err := c.Complete(rctx, system, user)
		cancel()
		if err == nil {
			return out, nil
		}
		if ctx.Err() != nil || !transcribe.Retryable(err) {
			return "", err
		}
		note := locales.StatusRetrying()
		if transcribe.Offline(err) {
			note = locales.StatusOffline()
		} else if tries++; tries >= maxTries {
			return "", err
		}
		q.status(st, id, library.StatusSummarizing, note)
		if !q.cfg.Sleep(ctx, backoff) {
			return "", ctx.Err()
		}
		backoff = min(backoff*2, 30*time.Second)
	}
}

func describe(err error) string {
	var he *transcribe.HTTPError
	switch {
	case errors.Is(err, netguard.ErrLocalOnly):
		return err.Error()
	case errors.As(err, &he) && (he.Status == 401 || he.Status == 403):
		return locales.ErrBadKey()
	}
	return locales.ErrSummarize(err.Error())
}

func find(all []Template, ok func(Template) bool) *Template {
	for i := range all {
		if ok(all[i]) {
			return &all[i]
		}
	}
	return nil
}
