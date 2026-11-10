package dsp

import "math"

const (
	// cutoffHz sits below every voice's fundamental and above desk knocks, footsteps and air-con hum.
	cutoffHz = 90
	// twoQ is the cookbook's 2·Q for Q = 1/√2, the Butterworth Q that gives the flattest pass band.
	twoQ = math.Sqrt2
)

// HighPass is a second-order Butterworth high-pass (RBJ Audio EQ Cookbook). State is float64 so a long recording
// does not drift.
type HighPass struct {
	b0, b1, b2, a1, a2 float64
	x1, x2, y1, y2     float64
}

func NewHighPass() *HighPass {
	w0 := 2 * math.Pi * cutoffHz / SampleRate
	cosW0 := math.Cos(w0)
	alpha := math.Sin(w0) / twoQ
	a0 := 1 + alpha
	return &HighPass{
		b0: (1 + cosW0) / 2 / a0,
		b1: -(1 + cosW0) / a0,
		b2: (1 + cosW0) / 2 / a0,
		a1: -2 * cosW0 / a0,
		a2: (1 - alpha) / a0,
	}
}

// Process filters frame in place.
func (h *HighPass) Process(frame []float32) {
	for i, s := range frame {
		x := float64(s)
		y := h.b0*x + h.b1*h.x1 + h.b2*h.x2 - h.a1*h.y1 - h.a2*h.y2
		h.x2, h.x1 = h.x1, x
		h.y2, h.y1 = h.y1, y
		frame[i] = float32(y)
	}
}
