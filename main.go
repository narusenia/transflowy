package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"

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
	mygo.BindAs("Settings", cfg)
	mygo.BindAs("Library", &Library{cfg: cfg})

	mygo.App.WhenReady(func() {
		mygo.NewWindow(mygo.WindowOptions{
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
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
