// Package summarize turns a finished transcript into Markdown summaries, one
// per template, through an OpenAI-compatible chat API.
package summarize

import (
	"embed"
	"os"
	"path/filepath"
	"strings"
)

//go:embed templates/*.md templates/en/*.md
var builtin embed.FS

// Template is a summary recipe: a Markdown file with a frontmatter header.
// Name is also the summary's file name (summary-<Name>.md).
type Template struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Builtin     bool   `json:"builtin"`
	Key         string `json:"key"` // a built-in's file name, the same in every language
	Prompt      string `json:"-"`
}

// parse reads "---\nkey: value\n---\nprompt". A file without a header is all prompt.
func parse(src, fallbackName string) Template {
	t := Template{Name: fallbackName, Prompt: strings.TrimSpace(src)}
	rest, ok := strings.CutPrefix(strings.ReplaceAll(src, "\r\n", "\n"), "---\n")
	if !ok {
		return t
	}
	head, body, ok := strings.Cut(rest, "\n---")
	if !ok {
		return t
	}
	for _, line := range strings.Split(head, "\n") {
		k, v, _ := strings.Cut(line, ":")
		switch strings.TrimSpace(k) {
		case "name":
			t.Name = strings.Trim(strings.TrimSpace(v), `"'`)
		case "description":
			t.Description = strings.Trim(strings.TrimSpace(v), `"'`)
		}
	}
	t.Prompt = strings.TrimSpace(strings.TrimPrefix(body, "\n"))
	// The name becomes a file name.
	t.Name = strings.TrimSpace(strings.NewReplacer("/", "-", "\\", "-").Replace(t.Name))
	t.Name = strings.TrimLeft(t.Name, ".")
	if t.Name == "" {
		t.Name = fallbackName
	}
	return t
}

// MemoKey is the built-in template for any kind of recording: the fallback
// when the chosen template does not exist in the app's language.
const MemoKey = "06-memo"

// Templates returns the built-in templates of the language (Japanese, else
// English) followed by the user's, which are the *.md files in
// <libraryDir>/templates. A user template with a built-in name replaces it.
func Templates(libraryDir, lang string) []Template {
	var out []Template
	dir := "templates/en"
	if strings.HasPrefix(lang, "ja") {
		dir = "templates"
	}
	entries, _ := builtin.ReadDir(dir)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		b, _ := builtin.ReadFile(dir + "/" + e.Name())
		t := parse(string(b), strings.TrimSuffix(e.Name(), ".md"))
		t.Builtin = true
		t.Key = strings.TrimSuffix(e.Name(), ".md")
		out = append(out, t)
	}
	users, _ := filepath.Glob(filepath.Join(libraryDir, "templates", "*.md"))
	for _, f := range users {
		b, err := os.ReadFile(f)
		if err != nil || strings.TrimSpace(string(b)) == "" {
			continue
		}
		t := parse(string(b), strings.TrimSuffix(filepath.Base(f), ".md"))
		replaced := false
		for i := range out {
			if out[i].Name == t.Name {
				out[i], replaced = t, true
			}
		}
		if !replaced {
			out = append(out, t)
		}
	}
	return out
}
