package transcribe

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"

	"transflowy/internal/library"
	"transflowy/internal/netguard"
)

// Request is one chunk: a 16 kHz mono WAV file.
type Request struct {
	WAV      []byte
	Language string   // "" detects
	Phrases  []string // names and terms to get right
	Diarize  bool
}

// Result has times relative to the start of the chunk.
type Result struct {
	Language string
	Segments []library.Segment
}

// Engine turns one chunk into text.
type Engine interface {
	Name() string
	Transcribe(ctx context.Context, r Request) (Result, error)
}

// HTTPError is a non-2xx answer.
type HTTPError struct {
	Status int
	Body   string
}

func (e *HTTPError) Error() string { return fmt.Sprintf("HTTP %d: %s", e.Status, e.Body) }

// Retryable reports whether trying the same chunk again can succeed: the
// network or the server failed, or it asked to slow down. A bad key or a
// refused file will not get better.
func Retryable(err error) bool {
	if errors.Is(err, netguard.ErrLocalOnly) || errors.Is(err, context.Canceled) {
		return false
	}
	var he *HTTPError
	if errors.As(err, &he) {
		return he.Status == 429 || he.Status >= 500
	}
	return isNetwork(err)
}

// Offline reports a failure to reach the server at all, which is waited out
// instead of counted against the chunk.
func Offline(err error) bool {
	var op *net.OpError
	var dns *net.DNSError
	return errors.As(err, &dns) || (errors.As(err, &op) && op.Op == "dial")
}

func isNetwork(err error) bool {
	var ne net.Error
	return errors.As(err, &ne) || errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "EOF")
}

// speakerOrUnknown maps a missing speaker to library.UnknownSpeaker.
func speakerOrUnknown(s *int) int {
	if s == nil {
		return library.UnknownSpeaker
	}
	return *s
}

// segment makes a transcript line; Original keeps the engine's text.
func segment(start, end float64, speaker int, text string) library.Segment {
	text = strings.TrimSpace(text)
	return library.Segment{Start: start, End: end, Speaker: speaker, Text: text, Original: text}
}
