package dsp_test

import (
	"math"
	"testing"

	"github.com/tienduc120/voice-recorder/services/recorder/internal/infrastructure/dsp"
)

// decimated runs a signal through a fresh decimator in 960-sample frames.
func decimated(in []float32) []float32 {
	d := dsp.NewDecimator()
	var out []float32
	for i := 0; i+960 <= len(in); i += 960 {
		out = d.Process(in[i:i+960], out)
	}
	return out
}

func TestDecimatorKeepsOneSampleInThree(t *testing.T) {
	t.Parallel()

	if got := len(dsp.NewDecimator().Process(make([]float32, 960), nil)); got != 320 {
		t.Errorf("960 samples in gave %d out, want 320: 48 kHz to 16 kHz", got)
	}
}

func TestDecimatorKeepsTheSpeechBand(t *testing.T) {
	t.Parallel()

	in := sine(1000, -20, 1)
	out := decimated(in)

	if got := rmsDB(out[len(out)/2:]); math.Abs(got-(-20)) > 0.5 {
		t.Errorf("1 kHz came out at %.2f dBFS, want -20 ± 0.5: speech passes untouched", got)
	}
}

func TestDecimatorRejectsWhatWouldFoldBack(t *testing.T) {
	t.Parallel()

	out := decimated(sine(10000, -20, 1))

	if got := rmsDB(out[len(out)/2:]); got > -60 {
		t.Errorf("10 kHz came out at %.1f dBFS, want at most -60 (40 dB down): it would alias to 6 kHz", got)
	}
}
