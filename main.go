package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"

	"transflowy/internal/library"
	"transflowy/internal/models"
	"transflowy/internal/pipeline"
	"transflowy/internal/settings"
	"transflowy/internal/summarize"
	"transflowy/internal/transcribe"
	"transflowy/locales"
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
	modelDir := filepath.Join(cfgDir, "transflowy", "models")
	server := &transcribe.Server{Model: models.Whisper.Path(modelDir)}
	engines := &Engines{cfg: cfg, server: server, modelDir: modelDir}
	store := func() (library.Store, error) { s, err := cfg.Get(); return library.Store{Root: s.LibraryDir}, err }
	changed := func(id string) { _ = LibraryChanged.Broadcast(id) }
	notify := func(id string) {
		st, err := store()
		if err != nil {
			return
		}
		r, err := st.Get(id)
		if err != nil || !mygo.NotificationsSupported() {
			return
		}
		_ = mygo.NewNotification(mygo.NotificationOptions{ID: "done-" + id, Title: locales.NotifyDone(), Body: r.Meta.Title}).Show()
	}
	sumQueue := summarize.New(summarize.Config{
		Store:     store,
		Templates: func() []summarize.Template { s, _ := cfg.Get(); return summarize.Templates(s.LibraryDir) },
		Default:   func() string { s, _ := cfg.Get(); return s.Template },
		Chat:      engines.Chat,
		Changed:   changed,
		Finished:  notify,
	})
	queue := pipeline.New(pipeline.Config{
		Store:       store,
		Engine:      engines.Build,
		Phrases:     phrases(cfg),
		Changed:     changed,
		Transcribed: func(id string) { sumQueue.Enqueue(id, "") },
	})
	ctx, stopQueue := context.WithCancel(context.Background())
	go queue.Run(ctx)
	go sumQueue.Run(ctx)
	// Settings and keys decide which engine is ready: recordings that waited are tried again.
	var refreshTray func()
	cfg.OnChange = func() {
		resumeAll(cfg, queue, sumQueue)
		if refreshTray != nil {
			refreshTray()
		}
	}

	rec := &Recorder{cfg: cfg, queue: queue}
	imp := &Importer{cfg: cfg, queue: queue}
	mygo.BindAs("Settings", cfg)
	mygo.BindAs("Library", &Library{cfg: cfg})
	mygo.BindAs("Recorder", rec)
	mygo.BindAs("Importer", imp)
	mygo.BindAs("Transcriber", &Transcriber{cfg: cfg, queue: queue})
	mygo.BindAs("Exporter", &Exporter{cfg: cfg})
	mygo.BindAs("Summarizer", &Summarizer{cfg: cfg, engines: engines, queue: sumQueue})
	mygo.BindAs("Models", &Models{dir: modelDir, onReady: func() { resumeAll(cfg, queue, sumQueue) }})

	// The page plays audio and reads peaks.json from the library through media://localhost/<id>/<file>.
	// The folder is read per request, so changing it in the settings needs no restart.
	if err := mygo.Protocol.HandleFunc("media", func(w http.ResponseWriter, r *http.Request) {
		s, _ := cfg.Get()
		mygo.FileServer(os.DirFS(s.LibraryDir)).ServeHTTP(w, r)
	}); err != nil {
		log.Println("media:", err)
	}

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
	mygo.App.OnWillQuit(func(*mygo.QuitEvent) {
		rec.shutdown()
		stopQueue()
		server.Stop()
	})

	mygo.App.WhenReady(func() {
		s, err := cfg.Get()
		if err != nil {
			log.Println("settings:", err)
		}
		recoverRecordings(library.Store{Root: s.LibraryDir})
		resumeAll(cfg, queue, sumQueue)

		locked := func() bool { s, _ := cfg.Get(); return s.LocalOnly }
		update, err := newTray(s.Shortcut, locked, rec.toggle, openWindow)
		if err != nil {
			log.Println("tray:", err)
		} else {
			rec.onState = update
			refreshTray = func() { update(rec.State()) }
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
