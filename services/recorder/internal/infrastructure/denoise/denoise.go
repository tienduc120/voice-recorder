// Package denoise adapts rnnoise-go, the noise cleanup, to the recorder's frames.
package denoise

import (
	"errors"
	"fmt"

	rnnoise "github.com/VoiceBlender/rnnoise-go"
	"github.com/VoiceBlender/rnnoise-go/model"

	"github.com/tienduc120/voice-recorder/services/recorder/internal/infrastructure/dsp"
)

const (
	// FrameSize is RNNoise's 10 ms frame at 48 kHz.
	FrameSize = 480
	// int16Scale is the sample scale the model was trained on; the pipeline runs at ±1.
	int16Scale = 32768
	// gainFloorDB caps how far any band is cut: uncapped cleanup cost transcript accuracy on heavy noise in the
	// spike, and the spec says to clean less when unsure (ADR-0018).
	gainFloorDB = -20
)

var ErrFrameSize = errors.New("denoise: a frame is 480 samples")

type Denoiser struct {
	rnn *rnnoise.Denoiser
	buf []float32
}

func New() (*Denoiser, error) {
	m, err := model.Load()
	if err != nil {
		return nil, fmt.Errorf("load the rnnoise model: %w", err)
	}
	rnn, err := rnnoise.New(rnnoise.Options{SampleRate: dsp.SampleRate, Model: m, GainFloorDB: gainFloorDB})
	if err != nil {
		return nil, fmt.Errorf("create the denoiser: %w", err)
	}
	if rnn.FrameSize() != FrameSize {
		return nil, fmt.Errorf("%w: rnnoise wants %d", ErrFrameSize, rnn.FrameSize())
	}
	return &Denoiser{rnn: rnn, buf: make([]float32, FrameSize)}, nil
}

// Process cleans one frame of samples in [-1, 1] in place.
func (d *Denoiser) Process(frame []float32) error {
	if len(frame) != FrameSize {
		return fmt.Errorf("%w: got %d", ErrFrameSize, len(frame))
	}
	for i, s := range frame {
		d.buf[i] = s * int16Scale
	}
	if _, err := d.rnn.Process(d.buf, d.buf); err != nil {
		return fmt.Errorf("denoise: %w", err)
	}
	for i, s := range d.buf {
		frame[i] = s / int16Scale
	}
	return nil
}
