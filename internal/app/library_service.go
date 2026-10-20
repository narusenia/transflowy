package app

import (
	"context"
	"github.com/egoist/mygo"
	"strings"

	"transflowy/internal/library"
	"transflowy/internal/settings"
	"transflowy/locales"
)

// Library is callable from the page: it reads recordings from the folder the
// settings name. Creating and changing recordings is done on the Go side.
type Library struct{ cfg *settings.Service }

func (l *Library) store() (library.Store, error) {
	s, err := l.cfg.Get()
	return library.Store{Root: s.LibraryDir}, err
}

// List returns every recording's meta, newest first.
func (l *Library) List() ([]library.Meta, error) {
	st, err := l.store()
	if err != nil {
		return nil, err
	}
	return st.List()
}

// Get returns one recording with its transcript.
func (l *Library) Get(id string) (library.Recording, error) {
	st, err := l.store()
	if err != nil {
		return library.Recording{}, err
	}
	return st.Get(id)
}

// Summary returns the Markdown of a recording's summary made with a template.
func (l *Library) Summary(id, template string) (string, error) {
	st, err := l.store()
	if err != nil {
		return "", err
	}
	return st.Summary(id, template)
}

// Search finds recordings by title, transcript and summary text.
func (l *Library) Search(query string) ([]library.Hit, error) {
	st, err := l.store()
	if err != nil {
		return nil, err
	}
	return st.Search(query, 50)
}

// ChooseFolder asks for a folder and returns it; "" when canceled.
func (l *Library) ChooseFolder() (string, error) {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Directory: true, CreateDirectories: true})
	if err != nil || len(paths) == 0 {
		return "", err
	}
	return paths[0], nil
}

// SetTitle renames a recording.
func (l *Library) SetTitle(id, title string) error {
	st, err := l.store()
	if err != nil {
		return err
	}
	return st.SetTitle(id, title)
}

// EditSegment changes the text of one transcript line. start is the line's
// start time as the page has it; the original engine text is kept.
func (l *Library) EditSegment(id string, index int, start float64, text string) error {
	st, err := l.store()
	if err != nil {
		return err
	}
	return st.EditSegment(id, index, start, text)
}

// ResetSegment puts the engine's original text back into one line.
func (l *Library) ResetSegment(id string, index int, start float64) error {
	st, err := l.store()
	if err != nil {
		return err
	}
	return st.ResetSegment(id, index, start)
}

// Replace changes every occurrence of find in a recording's transcript and
// returns how many lines changed.
func (l *Library) Replace(id, find, with string) (int, error) {
	st, err := l.store()
	if err != nil {
		return 0, err
	}
	return st.Replace(id, find, with)
}

// AddToDictionary adds a word to dictionary.txt, which the engines get.
func (l *Library) AddToDictionary(word string) error {
	st, err := l.store()
	if err != nil {
		return err
	}
	return st.AddToDictionary(word)
}

// LibraryMenu fires when a context-menu item needs the page: "rename".
var LibraryMenu = mygo.NewEvent[MenuAction]("libraryMenu")

// MenuAction names the recording and the item chosen.
type MenuAction struct {
	ID     string `json:"id"`
	Action string `json:"action"`
}

// ShowMenu shows the context menu of a recording at the mouse.
func (l *Library) ShowMenu(ctx context.Context, id string) {
	st, err := l.store()
	if err != nil {
		return
	}
	rec, err := st.Get(id)
	if err != nil {
		return
	}
	busy := rec.Meta.Status == library.StatusRecording || rec.Meta.Status == library.StatusImporting
	dir, _ := st.Path(id)
	menu := mygo.NewMenu([]*mygo.MenuItem{
		{Label: locales.MenuRename(), Click: func(*mygo.MenuItem, *mygo.Window) { _ = LibraryMenu.Broadcast(MenuAction{id, "rename"}) }},
		{Label: locales.MenuReveal(), Click: func(*mygo.MenuItem, *mygo.Window) { mygo.Shell.ShowItemInFolder(dir) }},
		mygo.Separator(),
		{Label: locales.MenuDelete(), Disabled: busy, Click: func(_ *mygo.MenuItem, win *mygo.Window) {
			go l.confirmDelete(win, rec.Meta, dir) // a dialog blocks: not on the menu's thread
		}},
	})
	menu.Popup(mygo.CallerWindow(ctx))
}

// ConfirmDelete asks, then moves a recording to the trash (the list's ✕ button).
func (l *Library) ConfirmDelete(ctx context.Context, id string) {
	st, err := l.store()
	if err != nil {
		return
	}
	rec, err := st.Get(id)
	if err != nil {
		return
	}
	dir, _ := st.Path(id)
	go l.confirmDelete(mygo.CallerWindow(ctx), rec.Meta, dir)
}

// confirmDelete moves a recording's folder to the trash after the user agrees.
func (l *Library) confirmDelete(win *mygo.Window, m library.Meta, dir string) {
	title := m.Title
	if title == "" {
		title = locales.Untitled()
	}
	res, err := mygo.Dialog.Message(mygo.MessageOptions{
		Parent: win, Type: mygo.MessageWarning, Message: locales.DeleteTitle(title), Detail: locales.DeleteDetail(),
		Buttons: []string{locales.DeleteConfirm(), locales.Cancel()}, DefaultButton: 1, CancelButton: 1,
	})
	if err != nil || res.Button != 0 {
		return
	}
	if err := mygo.Shell.TrashItem(dir); err != nil {
		mygo.Dialog.Error(title, err.Error())
		return
	}
	_ = LibraryChanged.Broadcast(m.ID)
}

// SetSpeakers names the speakers of a recording: speaker number (as a string)
// to name. Blank names go back to the default.
func (l *Library) SetSpeakers(id string, names map[string]string) error {
	st, err := l.store()
	if err != nil {
		return err
	}
	return st.SetSpeakers(id, names)
}

// SaveSummary stores a summary the user wrote or edited, under the name of a
// template or any other plain name. An empty text deletes it.
func (l *Library) SaveSummary(id, name, text string) error {
	st, err := l.store()
	if err != nil {
		return err
	}
	if strings.TrimSpace(text) == "" {
		return st.RemoveSummary(id, name)
	}
	return st.SaveSummary(id, name, strings.TrimSpace(text)+"\n")
}
