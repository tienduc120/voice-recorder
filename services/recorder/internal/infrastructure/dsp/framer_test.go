package dsp_test

import (
	"reflect"
	"testing"

	"github.com/tienduc120/voice-recorder/services/recorder/internal/infrastructure/dsp"
)

func ramp(from, to int) []float32 {
	out := make([]float32, 0, to-from)
	for v := from; v < to; v++ {
		out = append(out, float32(v))
	}
	return out
}

func TestFramerHandsOnExactFramesInOrder(t *testing.T) {
	t.Parallel()

	f := dsp.NewFramer(frameSize)
	var frames [][]float32
	collect := func(frame []float32) error {
		frames = append(frames, append([]float32(nil), frame...))
		return nil
	}

	for _, chunk := range [][]float32{ramp(0, 100), ramp(100, 500), ramp(500, 1000)} {
		if err := f.Push(chunk, collect); err != nil {
			t.Fatal(err)
		}
	}

	if want := [][]float32{ramp(0, 480), ramp(480, 960)}; !reflect.DeepEqual(frames, want) {
		t.Errorf("got %d frames, want exactly 0..479 and 480..959 in order", len(frames))
	}
	if got := f.Pending(); !reflect.DeepEqual(got, ramp(960, 1000)) {
		t.Errorf("pending = %v, want 960..999 waiting for the next frame", got)
	}
}
