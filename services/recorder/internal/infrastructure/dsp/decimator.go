package dsp

import "math"

const (
	// DecimationFactor takes 48 kHz to the 16 kHz that SILK wideband encodes.
	DecimationFactor = 3
	// decimatorTaps and the 7 kHz cutoff put everything that would fold back below 8 kHz at least 40 dB down.
	decimatorTaps     = 63
	decimatorCentre   = (decimatorTaps - 1) / 2
	decimatorCutoffHz = 7000
	// passBand is the cutoff as a fraction of the Nyquist frequency: the ideal low-pass's gain at its centre tap.
	passBand         = 2.0 * decimatorCutoffHz / SampleRate
	blackmanA0       = 0.42
	blackmanA1       = 0.5
	blackmanA2       = 0.08
	blackmanHarmonic = 2
)

// Decimator low-passes 48 kHz audio and keeps one sample in three. Its taps are a Blackman-windowed sinc.
type Decimator struct {
	taps []float64
	// hist holds the inputs the next call's first outputs still need.
	hist []float64
	x    []float64
}

func NewDecimator() *Decimator {
	taps := make([]float64, decimatorTaps)
	var sum float64
	for i := range taps {
		n := float64(i - decimatorCentre)
		ideal := passBand
		if n != 0 {
			ideal = math.Sin(math.Pi*passBand*n) / (math.Pi * n)
		}
		phase := math.Pi * float64(i) / decimatorCentre
		taps[i] = ideal * (blackmanA0 - blackmanA1*math.Cos(phase) + blackmanA2*math.Cos(blackmanHarmonic*phase))
		sum += taps[i]
	}
	for i := range taps {
		taps[i] /= sum
	}
	return &Decimator{taps: taps, hist: make([]float64, decimatorTaps-1)}
}

// Process appends len(in)/3 samples to out. len(in) must be a multiple of 3, so every call keeps the same phase.
func (d *Decimator) Process(in []float32, out []float32) []float32 {
	x := d.x[:0]
	x = append(x, d.hist...)
	for _, s := range in {
		x = append(x, float64(s))
	}
	past := len(d.hist)
	for i := DecimationFactor - 1; i < len(in); i += DecimationFactor {
		newest := i + past
		var y float64
		for k, t := range d.taps {
			y += t * x[newest-k]
		}
		out = append(out, float32(y))
	}
	copy(d.hist, x[len(x)-past:])
	d.x = x
	return out
}
