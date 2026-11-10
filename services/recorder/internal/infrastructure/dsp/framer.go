// Package dsp holds the signal stages before encoding: fixed frames, a rumble filter and a gentle leveller.
package dsp

// SampleRate is the one rate the pipeline runs at; RNNoise and Opus both need 48 kHz.
const SampleRate = 48000

// Framer turns samples of any length into frames of one size, because each later stage needs an exact frame:
// RNNoise 480 samples, Opus 960.
type Framer struct {
	size int
	buf  []float32
}

func NewFramer(size int) *Framer {
	return &Framer{size: size, buf: make([]float32, 0, size)}
}

// Push calls emit with every frame the samples complete. A frame is valid only during emit: its memory is reused.
func (f *Framer) Push(samples []float32, emit func(frame []float32) error) error {
	for len(samples) > 0 {
		n := min(f.size-len(f.buf), len(samples))
		f.buf = append(f.buf, samples[:n]...)
		samples = samples[n:]
		if len(f.buf) == f.size {
			if err := emit(f.buf); err != nil {
				return err
			}
			f.buf = f.buf[:0]
		}
	}
	return nil
}

// Pending is the samples waiting for a full frame.
func (f *Framer) Pending() []float32 { return f.buf }
