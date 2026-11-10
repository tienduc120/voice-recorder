package dsp_test

import (
	"math"
	"math/rand/v2"

	"github.com/tienduc120/voice-recorder/services/recorder/internal/infrastructure/dsp"
)

const frameSize = 480

func amplitude(rmsDBFS float64) float64 { return math.Pow(10, rmsDBFS/20) * math.Sqrt2 }

// sine returns seconds of a tone whose RMS is rmsDBFS.
func sine(freqHz, rmsDBFS, seconds float64) []float32 {
	return sineWithPeak(freqHz, amplitude(rmsDBFS), seconds)
}

func sineWithPeak(freqHz, peakValue, seconds float64) []float32 {
	out := make([]float32, int(seconds*dsp.SampleRate))
	for i := range out {
		out[i] = float32(peakValue * math.Sin(2*math.Pi*freqHz*float64(i)/dsp.SampleRate))
	}
	return out
}

func noise(rmsDBFS, seconds float64) []float32 {
	r := rand.New(rand.NewPCG(1, 2))
	scale := math.Pow(10, rmsDBFS/20)
	out := make([]float32, int(seconds*dsp.SampleRate))
	for i := range out {
		out[i] = float32(scale * r.NormFloat64())
	}
	return out
}

func rmsDB(s []float32) float64 {
	var sum float64
	for _, v := range s {
		sum += float64(v) * float64(v)
	}
	return 10 * math.Log10(sum/float64(len(s)))
}

func lastSecond(s []float32) []float32 { return s[len(s)-dsp.SampleRate:] }

// inFrames runs a stage over a copy of signal in 10 ms frames, as the recorder does.
func inFrames(stage func([]float32), signal []float32) []float32 {
	out := append([]float32(nil), signal...)
	for i := 0; i+frameSize <= len(out); i += frameSize {
		stage(out[i : i+frameSize])
	}
	return out
}
