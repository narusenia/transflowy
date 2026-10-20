package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"

	"github.com/egoist/mygo"

	"transflowy/locales"
)

// trayIcon draws the menu bar icon: a ring when idle, a filled dot while
// recording. Black on transparent, so macOS tints it for the menu bar.
func trayIcon(recording bool) []byte {
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
	var b bytes.Buffer
	_ = png.Encode(&b, img)
	return b.Bytes()
}

// newTray adds the menu bar item. toggle starts or stops a recording.
func newTray(shortcutLabel string, toggle, openWindow func()) (update func(RecordingState), err error) {
	rec := &mygo.MenuItem{ID: "record", Accelerator: shortcutLabel, Click: func(*mygo.MenuItem, *mygo.Window) { toggle() }}
	open := &mygo.MenuItem{Click: func(*mygo.MenuItem, *mygo.Window) { openWindow() }}
	quit := &mygo.MenuItem{Click: func(*mygo.MenuItem, *mygo.Window) { mygo.App.Quit() }}
	menu := mygo.NewMenu([]*mygo.MenuItem{rec, open, mygo.Separator(), quit})
	tray, err := mygo.NewTray(mygo.TrayOptions{Icon: trayIcon(false), IconIsTemplate: true, ToolTip: "transflowy", Menu: menu})
	if err != nil {
		return nil, err
	}
	var was bool
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
		if st.Recording != was { // the icon and the label change only on start and stop
			was = st.Recording
			_ = tray.SetIcon(trayIcon(st.Recording), true)
			labels()
		}
		if st.Recording {
			tray.SetTitle(fmt.Sprintf(" %d:%02d", int(st.Elapsed)/60, int(st.Elapsed)%60))
		} else {
			tray.SetTitle("")
		}
	}, nil
}
