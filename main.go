package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"

	"transflowy/internal/library"
	"transflowy/internal/settings"
)

func main() {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		log.Fatal(err)
	}
	cfg := &settings.Service{
		Path:           filepath.Join(cfgDir, "transflowy", "settings.json"),
		Home:           home,
		Keys:           mygo.Secrets,
		ErrKeyNotFound: mygo.ErrSecretNotFound,
	}
	rec := &Recorder{cfg: cfg}
	mygo.BindAs("Settings", cfg)
	mygo.BindAs("Library", &Library{cfg: cfg})
	imp := &Importer{cfg: cfg}
	mygo.BindAs("Recorder", rec)
	mygo.BindAs("Importer", imp)

	var win *mygo.Window
	openWindow := func() {
		if win != nil {
			win.Show()
			win.Focus()
			return
		}
		win = mygo.NewWindow(mygo.WindowOptions{
			Title:           "transflowy",
			Width:           1200,
			Height:          760,
			MinWidth:        720,
			MinHeight:       480,
			BackgroundColor: "light-dark(#ffffff, #0a0a0a)", // shadcn neutral --background
			// Opens where the user left it last time.
			StateKey: "main",
			// The frontend: devUrl during `mygo dev`, frontendDist
			// embedded by `mygo build` otherwise (see mygo.config.ts).
			URL: "/",
		})
		win.OnFileDrop(func(e *mygo.FileDropEvent) { imp.importFiles(e.Paths) })
	}

	// Closing the window must not end a recording: the app stays in the menu bar.
	mygo.App.OnWindowAllClosed(func() { win = nil })
	mygo.App.OnActivate(func(visible bool) {
		if !visible {
			openWindow()
		}
	})
	mygo.App.OnOpenFile(func(path string) { imp.importFiles([]string{path}) }) // Dock icon drop, "Open With"
	mygo.App.OnWillQuit(func(*mygo.QuitEvent) { rec.shutdown() })

	mygo.App.WhenReady(func() {
		s, err := cfg.Get()
		if err != nil {
			log.Println("settings:", err)
		}
		recoverRecordings(library.Store{Root: s.LibraryDir})

		update, err := newTray(s.Shortcut, rec.toggle, openWindow)
		if err != nil {
			log.Println("tray:", err)
		} else {
			rec.onState = update
		}
		if err := mygo.GlobalShortcut.Register(s.Shortcut, rec.toggle); err != nil {
			log.Println("shortcut:", err)
		}
		openWindow()
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
