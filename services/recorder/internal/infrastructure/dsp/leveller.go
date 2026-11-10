package dsp

import "math"

const (
	targetDBFS = -20
	// maxGainDB caps how far a quiet or distant speaker is lifted, so room noise is never lifted further.
	maxGainDB = 18
	minGainDB = -6
	// The gain rises slowly and falls fast: a quick rise pumps the room noise up between words, a slow fall lets
	// a loud word through too hot.
	riseDBPerSecond = 3
	fallDBPerSecond = 30
	// gateDBFS is the level below which a frame is silence or hiss, and the gain holds instead of chasing it.
	gateDBFS = -55
	// ceiling is -1 dBFS: the limiter keeps every sample below it.
	ceiling    = 0.891
	decibel    = 20
	decadeBase = 10
)

// Leveller is a gentle automatic gain: it lifts quiet speakers toward a target, never by more than maxGainDB, and
// a limiter stops anything from clipping.
type Leveller struct {
	gainDB float64
}

func NewLeveller() *Leveller { return &Leveller{} }

// Process levels frame in place; a frame of about 10 ms keeps the gain changes smooth.
func (l *Leveller) Process(frame []float32) {
	if len(frame) == 0 {
		return
	}
	from := l.gainDB
	if level := rmsDBFS(frame); level > gateDBFS {
		l.gainDB = step(from, clamp(targetDBFS-level, minGainDB, maxGainDB), float64(len(frame))/SampleRate)
	}
	applyRamp(frame, dbToGain(from), dbToGain(l.gainDB))
	limit(frame)
}

func step(from, want, seconds float64) float64 {
	if want > from {
		return math.Min(want, from+riseDBPerSecond*seconds)
	}
	return math.Max(want, from-fallDBPerSecond*seconds)
}

// applyRamp moves the gain across the frame instead of at its edge, so a change never clicks.
func applyRamp(frame []float32, from, to float64) {
	n := float64(len(frame))
	for i, s := range frame {
		g := from + (to-from)*float64(i+1)/n
		frame[i] = float32(float64(s) * g)
	}
}

func limit(frame []float32) {
	var peak float64
	for _, s := range frame {
		peak = math.Max(peak, math.Abs(float64(s)))
	}
	if peak <= ceiling {
		return
	}
	scale := ceiling / peak
	for i, s := range frame {
		frame[i] = float32(float64(s) * scale)
	}
}

func rmsDBFS(frame []float32) float64 {
	var sum float64
	for _, s := range frame {
		sum += float64(s) * float64(s)
	}
	return decibel / 2 * math.Log10(sum/float64(len(frame)))
}

func dbToGain(db float64) float64 { return math.Pow(decadeBase, db/decibel) }

func clamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }
