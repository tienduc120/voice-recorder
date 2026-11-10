package opusenc_test

import (
	"errors"
	"math"
	"math/rand/v2"
	"testing"

	"github.com/tienduc120/voice-recorder/services/recorder/internal/infrastructure/opusenc"
	"github.com/tienduc120/voice-recorder/services/recorder/internal/infrastructure/opusenc/opustest"
)

const (
	sampleRate = 48000
	maxPacket  = 4000
	frames     = 50
	maxLag     = 700
)

func newEncoder(t *testing.T) *opusenc.Encoder {
	t.Helper()
	e, err := opusenc.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = e.Close() })
	return e
}

// speechLike is tones across the speech band over a little noise, with a slow swell so it is not periodic.
func speechLike(seconds float64) []float32 {
	r := rand.New(rand.NewPCG(3, 4))
	out := make([]float32, int(seconds*sampleRate))
	for i := range out {
		x := float64(i) / sampleRate
		v := 0.15*math.Sin(2*math.Pi*350*x)*math.Sin(2*math.Pi*3*x) +
			0.08*math.Sin(2*math.Pi*1200*x+math.Sin(2*math.Pi*5*x)) + 0.02*math.Sin(2*math.Pi*3000*x)
		out[i] = float32(v + 0.01*r.NormFloat64())
	}
	return out
}

func encodeAll(t *testing.T, e *opusenc.Encoder, signal []float32) [][]byte {
	t.Helper()
	var packets [][]byte
	buf := make([]byte, maxPacket)
	for i := 0; i+opusenc.FrameSize <= len(signal); i += opusenc.FrameSize {
		n, err := e.Encode(signal[i:i+opusenc.FrameSize], buf)
		if err != nil {
			t.Fatal(err)
		}
		packets = append(packets, append([]byte(nil), buf[:n]...))
	}
	return packets
}

func decode(t *testing.T, packets [][]byte) []float64 {
	t.Helper()
	pcm, err := opustest.Decode(packets)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]float64, len(pcm))
	for i, v := range pcm {
		out[i] = float64(v) / 32768
	}
	return out
}

func rmsDB(s []float64) float64 {
	var sum float64
	for _, v := range s {
		sum += v * v
	}
	return 10 * math.Log10(sum/float64(len(s)))
}

func TestASecondOfSpeechBandAudioTakesAbout3KB(t *testing.T) {
	t.Parallel()

	total := 0
	for _, p := range encodeAll(t, newEncoder(t), speechLike(1)) {
		total += len(p)
	}

	if total < 2000 || total > 4500 {
		t.Errorf("one second encoded to %d bytes, want 2000–4500: 24 kbps is about 3000 bytes a second", total)
	}
}

func TestEncoderRefusesAWrongFrameSize(t *testing.T) {
	t.Parallel()

	_, err := newEncoder(t).Encode(make([]float32, opusenc.FrameSize/2), make([]byte, maxPacket))

	if !errors.Is(err, opusenc.ErrFrameSize) {
		t.Errorf("err = %v, want ErrFrameSize", err)
	}
}

func TestEncodedAudioDecodesBack(t *testing.T) {
	t.Parallel()

	in := speechLike(1)
	out := decode(t, encodeAll(t, newEncoder(t), in))

	inF := make([]float64, len(in))
	for i, v := range in {
		inF[i] = float64(v)
	}
	if len(out) != frames*opusenc.FrameSize {
		t.Fatalf("decoded %d samples, want %d", len(out), frames*opusenc.FrameSize)
	}
	if got, want := rmsDB(out[sampleRate/2:]), rmsDB(inF[sampleRate/2:]); math.Abs(got-want) > 6 {
		t.Errorf("decoded level %.1f dBFS, input %.1f: want within 6 dB", got, want)
	}
}

func TestPreSkipAlignsTheDecodedAudioWithTheInput(t *testing.T) {
	t.Parallel()

	e := newEncoder(t)
	in := speechLike(2)
	out := decode(t, encodeAll(t, e, in))[e.PreSkip():]

	best, bestCorr := 0, math.Inf(-1)
	for lag := -maxLag / 2; lag <= maxLag/2; lag++ {
		var xy, xx, yy float64
		for i := sampleRate / 2; i < len(in)-sampleRate/2; i++ {
			x, y := float64(in[i]), out[i+lag]
			xy += x * y
			xx += x * x
			yy += y * y
		}
		if c := xy / math.Sqrt(xx*yy); c > bestCorr {
			best, bestCorr = lag, c
		}
	}

	if best < -2 || best > 2 {
		t.Errorf("after the %d-sample pre-skip the audio lags by %d samples, want 0 ± 2", e.PreSkip(), best)
	}
}

func TestCloseIsSafe(t *testing.T) {
	t.Parallel()

	e, err := opusenc.New()
	if err != nil {
		t.Fatal(err)
	}

	if err = e.Close(); err != nil {
		t.Errorf("close = %v, want nil", err)
	}
}
