package capture

import (
	"testing"
	"time"
)

func TestAStreamSilentForThreeSecondsCountsAsStalled(t *testing.T) {
	t.Parallel()

	opened := time.Date(2026, time.November, 11, 10, 0, 0, 0, time.UTC)
	at := func(ms int) time.Time { return opened.Add(time.Duration(ms) * time.Millisecond) }
	w := newStallWatch(opened)

	if w.observe(0, at(2999)) {
		t.Error("stalled after 2.999 s: a slow start is not a stall")
	}
	if w.observe(4, at(3000)) {
		t.Error("stalled on a drain that brought packets")
	}
	if w.observe(0, at(5999)) {
		t.Error("stalled 2.999 s after the last packet")
	}
	if !w.observe(0, at(6000)) {
		t.Error(
			"not stalled 3 s after the last packet: a hung driver would record nothing while the page says Recording",
		)
	}
}
