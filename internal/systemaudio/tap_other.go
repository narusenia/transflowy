//go:build !darwin

package systemaudio

import "errors"

// Supported reports whether this build can capture system audio.
const Supported = false

func startTap(func([]float32, int)) (func(), float64, int, error) {
	return nil, 0, 0, errors.New("system audio capture is not supported on this OS")
}
