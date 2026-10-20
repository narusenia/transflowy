//go:build !darwin

package audio

import "errors"

var errUnsupported = errors.New("audio conversion is only implemented on macOS")

// ponytail: Windows needs Media Foundation or a bundled ffmpeg.
func ToWAV16k(src, dst string) error { return errUnsupported }
func ToM4A(src, dst string) error    { return errUnsupported }
