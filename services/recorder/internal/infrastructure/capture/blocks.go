package capture

import (
	"slices"

	"github.com/tienduc120/voice-recorder/services/recorder/internal/infrastructure/dsp"
)

// blockSize is the 10 ms block the recorder consumes, whatever packet size the device delivers.
const blockSize = 480

// blocker re-chunks device packets into blocks and hands them on without ever blocking the device's thread: when
// the recorder falls behind, a block is dropped and counted rather than queued without bound.
type blocker struct {
	framer *dsp.Framer
	frames chan<- []float32
	drops  uint64
}

func newBlocker(frames chan<- []float32) *blocker {
	return &blocker{framer: dsp.NewFramer(blockSize), frames: frames}
}

func (b *blocker) push(samples []float32) {
	_ = b.framer.Push(samples, func(block []float32) error {
		select {
		case b.frames <- slices.Clone(block):
		default:
			b.drops++
		}
		return nil
	})
}

func (b *blocker) dropped() uint64 { return b.drops }
