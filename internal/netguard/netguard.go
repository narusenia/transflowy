// Package netguard is the one way out for requests that carry the user's
// recordings or transcripts. With local-only on, it refuses every host that is
// not this machine, so a cloud engine cannot be reached by mistake.
package netguard

import (
	"net"
	"net/http"

	"transflowy/locales"
)

// LocalOnlyError is returned for a request to another machine while
// local-only is on. Test for it with errors.As.
type LocalOnlyError struct{}

func (LocalOnlyError) Error() string { return locales.ErrLocalOnly() }

// ErrLocalOnly is a LocalOnlyError, for errors.Is.
var ErrLocalOnly error = LocalOnlyError{}

type guard struct {
	localOnly func() bool
	next      http.RoundTripper
}

// Client returns an HTTP client that checks localOnly before every request,
// redirects included.
func Client(localOnly func() bool) *http.Client {
	return &http.Client{Transport: guard{localOnly, http.DefaultTransport}}
}

func (g guard) RoundTrip(r *http.Request) (*http.Response, error) {
	if g.localOnly() && !isLoopback(r.URL.Hostname()) {
		return nil, ErrLocalOnly
	}
	return g.next.RoundTrip(r)
}

// isLoopback: only this machine counts. A LAN address is another machine.
func isLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// Allowed reports whether a URL may be used now, for the settings screen to
// grey out engines before a request fails.
func Allowed(localOnly bool, host string) bool { return !localOnly || isLoopback(host) }
