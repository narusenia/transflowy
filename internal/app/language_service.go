package app

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/i18n"

	"transflowy/internal/settings"
)

// Language is callable from the settings: the language the app shows.
type Language struct{ cfg *settings.Service }

// LanguageOption is a language the app is written in, named in itself.
type LanguageOption struct {
	Tag  string `json:"tag"`
	Name string `json:"name"`
}

// Options lists the languages the app is written in.
func (*Language) Options() []LanguageOption {
	var out []LanguageOption
	for _, tag := range mygo.App.Languages() {
		out = append(out, LanguageOption{tag, i18n.Autonym(tag)})
	}
	return out
}

// Set shows another language at once and keeps the choice; "" follows the
// system's languages.
func (l *Language) Set(tag string) error {
	cur, err := l.cfg.Get()
	if err != nil {
		return err
	}
	cur.Language = tag
	if err := l.cfg.Set(cur); err != nil {
		return err
	}
	mygo.App.SetLocale(tag)
	return nil
}
