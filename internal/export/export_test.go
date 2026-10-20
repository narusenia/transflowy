package export

import (
	"strings"
	"testing"

	"transflowy/internal/library"
)

var rec = library.Recording{
	Meta: library.Meta{Title: "週次"},
	Transcript: library.Transcript{
		Segments: []library.Segment{
			{Start: 5, End: 9.5, Speaker: library.UnknownSpeaker, Text: "はじめます"},
			{Start: 4500, End: 4500, Speaker: 1, Text: "予算"},
		},
		Speakers: map[string]string{"1": "田中"},
	},
}

func TestLines(t *testing.T) {
	if got := Lines(rec.Transcript, true); got != "[00:05] はじめます\n[75:00] 田中: 予算\n" {
		t.Fatalf("%q", got)
	}
	if got := Lines(rec.Transcript, false); got != "はじめます\n田中: 予算\n" {
		t.Fatalf("%q", got)
	}
}

func TestText(t *testing.T) {
	if got := Text(rec, "## 概要\n要点\n", Summary, true); got != "## 概要\n要点\n" {
		t.Fatalf("%q", got)
	}
	both := Text(rec, "要点", Both, false)
	if !strings.HasPrefix(both, "# 週次\n\n要点\n\n## ") || !strings.HasSuffix(both, "はじめます\n田中: 予算\n") {
		t.Fatalf("%q", both)
	}
}

func TestSRT(t *testing.T) {
	want := "1\n00:00:05,000 --> 00:00:09,500\nはじめます\n\n2\n01:15:00,000 --> 01:15:02,000\n田中: 予算\n\n"
	if got := SRT(rec.Transcript); got != want {
		t.Fatalf("%q", got)
	}
}
