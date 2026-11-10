package dsp_test

import (
	"math"
	"testing"

	"github.com/tienduc120/voice-recorder/services/recorder/internal/infrastructure/dsp"
)

func levelled(signal []float32) []float32 {
	return inFrames(dsp.NewLeveller().Process, signal)
}

func TestLevellerLiftsAQuietSpeakerOnlyUpToTheCap(t *testing.T) {
	t.Parallel()

	got := rmsDB(lastSecond(levelled(sine(1000, -40, 20))))

	if math.Abs(got-(-22)) > 0.5 {
		t.Errorf("output = %.2f dBFS, want -22 ± 0.5: lifted by the 18 dB cap, not all the way to -20", got)
	}
}

func TestLevellerBringsAModerateSpeakerToTheTarget(t *testing.T) {
	t.Parallel()

	got := rmsDB(lastSecond(levelled(sine(1000, -30, 20))))

	if math.Abs(got-(-20)) > 1 {
		t.Errorf("output = %.2f dBFS, want -20 ± 1", got)
	}
}

func TestLevellerDoesNotRaiseSilence(t *testing.T) {
	t.Parallel()

	in := noise(-70, 20)

	if got, before := rmsDB(levelled(in)), rmsDB(in); got > before+0.5 {
		t.Errorf("output %.2f dBFS from %.2f: silence must not become loud hiss", got, before)
	}
}

func TestLevellerNeverClips(t *testing.T) {
	t.Parallel()

	in := append(sine(1000, -40, 10), sineWithPeak(1000, 1, 1)...)

	for i, v := range levelled(in) {
		if math.Abs(float64(v)) > 0.891 {
			t.Fatalf("sample %d = %v, want every sample within -1 dBFS (0.891): a loud word after quiet ones",
				i, v)
		}
	}
}
