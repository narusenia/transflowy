// Package export turns a recording into text for the clipboard and for files.
package export

import (
	"fmt"
	"strconv"
	"strings"

	"transflowy/internal/library"
	"transflowy/locales"
)

// What a text holds.
const (
	Summary    = "summary"
	Transcript = "transcript"
	Both       = "both"
)

func speaker(t library.Transcript, n int) string {
	if n == library.UnknownSpeaker {
		return ""
	}
	if name := t.Speakers[strconv.Itoa(n)]; name != "" {
		return name
	}
	return locales.SpeakerLabel(n + 1) // speaker 0 is 話者1, as the page shows it
}

// Lines renders a transcript as "[12:34] 話者1: text" lines, without the
// stamps when stamps is false. Minutes are not wrapped at 60: [75:00].
func Lines(t library.Transcript, stamps bool) string {
	var b strings.Builder
	for _, s := range t.Segments {
		if stamps {
			sec := int(s.Start)
			fmt.Fprintf(&b, "[%02d:%02d] ", sec/60, sec%60)
		}
		if name := speaker(t, s.Speaker); name != "" {
			b.WriteString(name + ": ")
		}
		b.WriteString(s.Text + "\n")
	}
	return b.String()
}

// Text renders what include names: the summary, the transcript, or the title,
// summary and transcript as one Markdown document.
func Text(r library.Recording, summary, include string, stamps bool) string {
	summary = strings.TrimSpace(summary)
	switch include {
	case Summary:
		return summary + "\n"
	case Transcript:
		return Lines(r.Transcript, stamps)
	}
	var b strings.Builder
	if r.Meta.Title != "" {
		b.WriteString("# " + r.Meta.Title + "\n\n")
	}
	if summary != "" {
		b.WriteString(summary + "\n\n")
	}
	b.WriteString("## " + locales.ExportTranscript() + "\n\n" + Lines(r.Transcript, stamps))
	return b.String()
}

// SRT renders the transcript as subtitles. A line ends where the engine says,
// or where the next one starts when it gave no usable end.
func SRT(t library.Transcript) string {
	var b strings.Builder
	for i, s := range t.Segments {
		end := s.End
		if end <= s.Start {
			end = s.Start + 2
			if i+1 < len(t.Segments) {
				end = max(s.Start+0.5, min(end, t.Segments[i+1].Start))
			}
		}
		text := s.Text
		if name := speaker(t, s.Speaker); name != "" {
			text = name + ": " + text
		}
		fmt.Fprintf(&b, "%d\n%s --> %s\n%s\n\n", i+1, srtTime(s.Start), srtTime(end), text)
	}
	return b.String()
}

func srtTime(sec float64) string {
	ms := int(sec*1000 + 0.5)
	return fmt.Sprintf("%02d:%02d:%02d,%03d", ms/3600000, ms/60000%60, ms/1000%60, ms%1000)
}
