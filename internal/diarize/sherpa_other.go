//go:build !darwin

package diarize

import "errors"

// Supported reports whether this build can diarize.
const Supported = false

// Config names the two models and how finely voices are told apart.
type Config struct {
	Segmentation, Embedding string
	NumSpeakers             int
	Threshold               float32
}

// DefaultThreshold is sherpa-onnx's suggestion for these models.
const DefaultThreshold = 0.7

// Run is not available on this OS yet.
func Run(Config, []int16) ([]Turn, error) {
	return nil, errors.New("local speaker diarization is not supported on this OS")
}
