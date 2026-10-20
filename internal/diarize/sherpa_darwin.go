package diarize

import (
	"errors"
	"runtime"

	sherpa "github.com/k2-fsa/sherpa-onnx-go-macos"
)

// Supported reports whether this build can diarize.
const Supported = true

// Config names the two models and how finely voices are told apart.
type Config struct {
	Segmentation, Embedding string
	// NumSpeakers fixes the number of speakers; 0 finds it out with Threshold.
	NumSpeakers int
	// Threshold is the clustering distance: lower splits voices more
	// (more speakers), higher merges them.
	Threshold float32
}

// DefaultThreshold is sherpa-onnx's suggestion for these models.
const DefaultThreshold = 0.5

// Run diarizes 16 kHz mono samples. It takes about a tenth of the audio's
// length on an Apple silicon Mac, and holds the whole recording in memory.
func Run(cfg Config, pcm []int16) ([]Turn, error) {
	if len(pcm) == 0 {
		return nil, nil
	}
	if cfg.Threshold <= 0 {
		cfg.Threshold = DefaultThreshold
	}
	threads := max(2, runtime.NumCPU()/2)
	sd := sherpa.NewOfflineSpeakerDiarization(&sherpa.OfflineSpeakerDiarizationConfig{
		Segmentation: sherpa.OfflineSpeakerSegmentationModelConfig{
			Pyannote:   sherpa.OfflineSpeakerSegmentationPyannoteModelConfig{Model: cfg.Segmentation},
			NumThreads: threads, Provider: "cpu",
		},
		Embedding:      sherpa.SpeakerEmbeddingExtractorConfig{Model: cfg.Embedding, NumThreads: threads, Provider: "cpu"},
		Clustering:     sherpa.FastClusteringConfig{NumClusters: cfg.NumSpeakers, Threshold: cfg.Threshold},
		MinDurationOn:  0.3,
		MinDurationOff: 0.5,
	})
	if sd == nil {
		return nil, errors.New("diarization: the models could not be loaded")
	}
	defer sherpa.DeleteOfflineSpeakerDiarization(sd)
	samples := make([]float32, len(pcm))
	for i, v := range pcm {
		samples[i] = float32(v) / 32768
	}
	segs := sd.Process(samples)
	turns := make([]Turn, len(segs))
	for i, s := range segs {
		turns[i] = Turn{Start: float64(s.Start), End: float64(s.End), Speaker: s.Speaker}
	}
	return turns, nil
}
