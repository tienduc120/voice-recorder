package capture

import (
	"reflect"
	"testing"
)

func ramp(from, to int) []float32 {
	out := make([]float32, 0, to-from)
	for v := from; v < to; v++ {
		out = append(out, float32(v))
	}
	return out
}

func TestBlockerHandsOnWholeBlocksAndDropsWhenFull(t *testing.T) {
	t.Parallel()

	frames := make(chan []float32, 1)
	b := newBlocker(frames)

	b.push(ramp(0, 1500))

	if got := <-frames; !reflect.DeepEqual(got, ramp(0, 480)) || b.dropped() != 2 {
		t.Fatalf("first block %v…, dropped %d: want 0..479 delivered and the two blocks that did not fit dropped",
			got[:3], b.dropped())
	}

	b.push(ramp(1500, 1920))

	if got := <-frames; !reflect.DeepEqual(got, ramp(1440, 1920)) {
		t.Errorf("next block starts %v, want 1440..1919: the samples left over wait for the next push", got[:3])
	}
}
