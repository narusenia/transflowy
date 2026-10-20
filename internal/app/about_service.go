package app

import "github.com/egoist/mygo"

const repository = "https://github.com/narusenia/transflowy"

// About is callable from the About dialog.
type About struct{}

// Version is the version of the packaged app; empty when it runs in development.
func (*About) Version() string { return mygo.App.Version() }

// OpenRepository opens the project page in the browser.
func (*About) OpenRepository() error { return mygo.Shell.OpenExternal(repository) }
