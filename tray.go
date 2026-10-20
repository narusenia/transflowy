package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"sync"

	"github.com/egoist/mygo"

	"transflowy/locales"
)

// trayIcon draws the menu bar icon: a ring when idle, a filled dot while
// recording. Black on transparent, so macOS tints it for the menu bar.
func trayIcon(recording, locked bool) []byte {
	const size, c, outer, inner = 32, 15.5, 11.0, 7.0
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx, dy := float64(x)-c, float64(y)-c
			d := dx*dx + dy*dy
			if d <= outer*outer && (recording || d >= inner*inner) {
				img.SetNRGBA(x, y, color.NRGBA{A: 255})
			}
		}
	}
	if locked { // a padlock in the lower right corner: nothing leaves this machine
		for y := 17; y < size; y++ {
			for x := 18; x < size; x++ {
				dx, dy := float64(x)-24, float64(y)-19
				body := y >= 21 && x >= 19 && x <= 29
				shackle := y <= 21 && dx*dx+dy*dy >= 9 && dx*dx+dy*dy <= 25
				if body || shackle {
					img.SetNRGBA(x, y, color.NRGBA{A: 255})
				}
			}
		}
	}
	var b bytes.Buffer
	_ = png.Encode(&b, img)
	return b.Bytes()
}

// newTray adds the menu bar item. toggle starts or stops a recording.
func newTray(shortcutLabel string, locked func() bool, toggle, openWindow func()) (update func(RecordingState), err error) {
	rec := &mygo.MenuItem{ID: "record", Accelerator: shortcutLabel, Click: func(*mygo.MenuItem, *mygo.Window) { toggle() }}
	open := &mygo.MenuItem{Click: func(*mygo.MenuItem, *mygo.Window) { openWindow() }}
	quit := &mygo.MenuItem{Click: func(*mygo.MenuItem, *mygo.Window) { mygo.App.Quit() }}
	menu := mygo.NewMenu([]*mygo.MenuItem{rec, open, mygo.Separator(), quit})
	tray, err := mygo.NewTray(mygo.TrayOptions{Icon: trayIcon(false, locked()), IconIsTemplate: true, ToolTip: "transflowy", Menu: menu})
	if err != nil {
		return nil, err
	}
	var mu sync.Mutex // the recorder's ticker and a settings change both update the tray
	var was, wasLocked bool
	wasLocked = locked()
	// labels sets the texts of the menu: custom menus are not translated by
	// mygo, so this runs again when the language changes.
	labels := func() {
		if was {
			rec.SetLabel(locales.TrayRecordStop())
		} else {
			rec.SetLabel(locales.TrayRecordStart())
		}
		open.SetLabel(locales.TrayOpenWindow())
		quit.SetLabel(locales.TrayQuit())
	}
	labels()
	mygo.App.OnLocaleChanged(func(string) { labels() })
	return func(st RecordingState) {
		mu.Lock()
		defer mu.Unlock()
		if l := locked(); st.Recording != was || l != wasLocked { // the icon and the label change only on start, stop and local-only
			was, wasLocked = st.Recording, l
			_ = tray.SetIcon(trayIcon(st.Recording, l), true)
			labels()
		}
		if st.Recording {
			tray.SetTitle(fmt.Sprintf(" %d:%02d", int(st.Elapsed)/60, int(st.Elapsed)%60))
		} else {
			tray.SetTitle("")
		}
	}, nil
}
