package denoise_test

import (
	"errors"
	"math"
	"math/rand/v2"
	"testing"

	"github.com/tienduc120/voice-recorder/services/recorder/internal/infrastructure/denoise"
)

const sampleRate = 48000

func newDenoiser(t *testing.T) *denoise.Denoiser {
	t.Helper()
	d, err := denoise.New()
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func rmsDB(s []float32) float64 {
	var sum float64
	for _, v := range s {
		sum += float64(v) * float64(v)
	}
	return 10 * math.Log10(sum/float64(len(s)))
}

func process(t *testing.T, d *denoise.Denoiser, signal []float32) []float32 {
	t.Helper()
	out := append([]float32(nil), signal...)
	for i := 0; i+denoise.FrameSize <= len(out); i += denoise.FrameSize {
		if err := d.Process(out[i : i+denoise.FrameSize]); err != nil {
			t.Fatal(err)
		}
	}
	return out
}

func TestDenoiserQuietsSteadyNoise(t *testing.T) {
	t.Parallel()

	r := rand.New(rand.NewPCG(1, 2))
	scale := math.Pow(10, -30.0/20)
	in := make([]float32, 3*sampleRate)
	for i := range in {
		in[i] = float32(scale * r.NormFloat64())
	}

	out := process(t, newDenoiser(t), in)

	settled := sampleRate
	if got, before := rmsDB(out[settled:]), rmsDB(in[settled:]); got > before-10 {
		t.Errorf("noise at %.1f dBFS came out at %.1f, want at least 10 dB quieter", before, got)
	}
}

func TestDenoiserKeepsSilenceSilent(t *testing.T) {
	t.Parallel()

	out := process(t, newDenoiser(t), make([]float32, sampleRate))

	for i, v := range out {
		if math.Abs(float64(v)) >= 1e-4 {
			t.Fatalf("sample %d = %v, want silence to stay silent", i, v)
		}
	}
}

func TestDenoiserRefusesAWrongFrameSize(t *testing.T) {
	t.Parallel()

	err := newDenoiser(t).Process(make([]float32, denoise.FrameSize-1))

	if !errors.Is(err, denoise.ErrFrameSize) {
		t.Errorf("err = %v, want ErrFrameSize", err)
	}
}
