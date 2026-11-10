package dsp_test

import (
	"math"
	"testing"

	"github.com/tienduc120/voice-recorder/services/recorder/internal/infrastructure/dsp"
)

// gainAt is the filter's steady-state gain in dB at one frequency, measured after the start-up transient.
func gainAt(freqHz float64) float64 {
	in := sine(freqHz, -20, 2)
	out := inFrames(dsp.NewHighPass().Process, in)
	return rmsDB(lastSecond(out)) - rmsDB(lastSecond(in))
}

func TestHighPassCutsRumble(t *testing.T) {
	t.Parallel()

	if g := gainAt(30); g > -18 {
		t.Errorf("30 Hz gain = %.1f dB, want at most -18: desk knocks and air-con hum must go", g)
	}
}

func TestHighPassKeepsTheSpeechBand(t *testing.T) {
	t.Parallel()

	if g := gainAt(1000); math.Abs(g) > 0.5 {
		t.Errorf("1 kHz gain = %.2f dB, want within 0.5 dB: speech must pass untouched", g)
	}
}

func TestHighPassCornerIsNear90Hz(t *testing.T) {
	t.Parallel()

	if g := gainAt(90); g < -4 || g > -2 {
		t.Errorf("90 Hz gain = %.2f dB, want about -3 dB: the corner sits between 80 and 100 Hz", g)
	}
}
