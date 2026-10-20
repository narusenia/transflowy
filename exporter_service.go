package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"

	"transflowy/internal/export"
	"transflowy/internal/library"
	"transflowy/internal/settings"
)

// Exporter is callable from the page.
type Exporter struct{ cfg *settings.Service }

// render builds the text of a recording. include is "summary", "transcript"
// or "both"; template names the summary to use.
func (e *Exporter) render(id, template, include string, stamps bool) (library.Recording, string, error) {
	cur, err := e.cfg.Get()
	if err != nil {
		return library.Recording{}, "", err
	}
	st := library.Store{Root: cur.LibraryDir}
	rec, err := st.Get(id)
	if err != nil {
		return rec, "", err
	}
	summary := ""
	if include != export.Transcript {
		if summary, err = st.Summary(id, template); err != nil && include == export.Summary {
			return rec, "", err
		}
	}
	return rec, export.Text(rec, summary, include, stamps), nil
}

// Text returns the text for the clipboard.
func (e *Exporter) Text(id, template, include string, stamps bool) (string, error) {
	_, text, err := e.render(id, template, include, stamps)
	return text, err
}

// Save asks where to put a .md, .txt or .srt file and writes it. It returns
// the path, or "" when the dialog was canceled. A .txt and a .srt hold the
// transcript; a .md holds what include names.
func (e *Exporter) Save(id, template, format, include string, stamps bool) (string, error) {
	var text string
	var rec library.Recording
	var err error
	switch format {
	case "md":
		rec, text, err = e.render(id, template, include, stamps)
	case "txt":
		rec, text, err = e.render(id, template, export.Transcript, stamps)
	case "srt":
		if rec, _, err = e.render(id, template, export.Transcript, stamps); err == nil {
			text = export.SRT(rec.Transcript)
		}
	default:
		return "", errors.New("unknown format")
	}
	if err != nil {
		return "", err
	}
	name := strings.NewReplacer("/", "-", "\\", "-", ":", "-").Replace(rec.Meta.Title)
	if name == "" {
		name = rec.Meta.ID
	}
	path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
		DefaultPath: name + "." + format,
		Filters:     []mygo.FileFilter{{Name: strings.ToUpper(format), Extensions: []string{format}}},
	})
	if err != nil || path == "" {
		return "", err
	}
	if filepath.Ext(path) == "" {
		path += "." + format
	}
	return path, os.WriteFile(path, []byte(text), 0o644)
}
