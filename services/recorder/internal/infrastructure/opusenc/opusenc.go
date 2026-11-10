// Package opusenc encodes the recorder's 48 kHz frames as Opus SILK wideband, Opus's speech codec (ADR-0019).
package opusenc

import (
	"errors"
	"fmt"
	"math"

	gopus "github.com/skrashevich/go-opus"

	"github.com/tienduc120/voice-recorder/services/recorder/internal/infrastructure/dsp"
)

const (
	// FrameSize is one 20 ms Opus frame at 48 kHz; SILK wideband sees it as 320 samples at 16 kHz.
	FrameSize = 960
	silkFrame = FrameSize / dsp.DecimationFactor
	// bitrate gives about 11 MB an hour of speech that speech-to-text reads as well as the original (ADR-0019).
	bitrate    = 24000
	complexity = 10
	// preSkip is the whole encode-decode delay at 48 kHz, decimator included, so decoded audio lines up with what
	// the mic heard; TestPreSkipAlignsTheDecodedAudioWithTheInput pins it.
	preSkip = 338
	vendor  = "go-opus v0.1.1 (libopus, SILK wideband)"
)

var ErrFrameSize = errors.New("opusenc: a frame is 960 samples")

type Encoder struct {
	enc  *gopus.Encoder
	dec  *dsp.Decimator
	down []float32
	pcm  []int16
}

func New() (*Encoder, error) {
	enc, err := gopus.NewEncoder(
		gopus.WithSampleRate(dsp.SampleRate),
		gopus.WithChannels(1),
		gopus.WithBitrate(bitrate),
		gopus.WithVBR(true),
		gopus.WithComplexity(complexity),
	)
	if err != nil {
		return nil, fmt.Errorf("create the opus encoder: %w", err)
	}
	return &Encoder{
		enc:  enc,
		dec:  dsp.NewDecimator(),
		down: make([]float32, 0, silkFrame),
		pcm:  make([]int16, silkFrame),
	}, nil
}

// Encode encodes one frame of samples in [-1, 1] into out and returns the packet's length.
func (e *Encoder) Encode(pcm []float32, out []byte) (int, error) {
	if len(pcm) != FrameSize {
		return 0, fmt.Errorf("%w: got %d", ErrFrameSize, len(pcm))
	}
	e.down = e.dec.Process(pcm, e.down[:0])
	for i, s := range e.down {
		e.pcm[i] = toInt16(s)
	}
	n, err := e.enc.EncodeSILK(e.pcm, gopus.BandwidthWideband, out)
	if err != nil {
		return 0, fmt.Errorf("encode: %w", err)
	}
	return n, nil
}

// PreSkip is the number of 48 kHz samples a player discards at the start (RFC 7845 §4.2).
func (e *Encoder) PreSkip() int { return preSkip }

func (e *Encoder) Vendor() string { return vendor }

// Close has nothing to release: go-opus frees its codec memory when the encoder is collected.
func (e *Encoder) Close() error { return nil }

func toInt16(s float32) int16 {
	v := math.Round(float64(s) * math.MaxInt16)
	return int16(math.Max(math.MinInt16, math.Min(math.MaxInt16, v)))
}
