package audiofile_test

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/tienduc120/voice-recorder/services/recorder/internal/application"
	"github.com/tienduc120/voice-recorder/services/recorder/internal/infrastructure/audiofile"
	"github.com/tienduc120/voice-recorder/services/recorder/internal/infrastructure/oggopus/oggread"
	"github.com/tienduc120/voice-recorder/services/recorder/internal/infrastructure/opusenc/opustest"
)

const (
	sampleRate = 48000
	frame      = 960
	fakePre    = 312
	flagEOS    = 0x04
	// realPre is opusenc's pre-skip, pinned by its own test.
	realPre = 338
)

type fakeEncoder struct {
	frames []int
	closes int
}

func (e *fakeEncoder) Encode(pcm []float32, out []byte) (int, error) {
	e.frames = append(e.frames, len(pcm))
	out[0] = 1
	return 1, nil
}

func (e *fakeEncoder) PreSkip() int { return fakePre }

func (e *fakeEncoder) Vendor() string { return "fake" }

func (e *fakeEncoder) Close() error {
	e.closes++
	return nil
}

type fakeDenoiser struct{ frames []int }

func (d *fakeDenoiser) Process(frame []float32) error {
	d.frames = append(d.frames, len(frame))
	return nil
}

func tone(seconds float64) []float32 {
	out := make([]float32, int(seconds*sampleRate))
	amp := math.Pow(10, -20.0/20) * math.Sqrt2
	for i := range out {
		out[i] = float32(amp * math.Sin(2*math.Pi*440*float64(i)/sampleRate))
	}
	return out
}

func create(t *testing.T, files *audiofile.Files, cleanup bool) (application.AudioFile, string) {
	t.Helper()
	dir := t.TempDir()
	opts := application.AudioOptions{NoiseCleanup: cleanup}
	f, err := files.Create(dir, func(int) string { return "rec.ogg" }, opts)
	if err != nil {
		t.Fatal(err)
	}
	return f, filepath.Join(dir, "rec.ogg")
}

func write(t *testing.T, f application.AudioFile, samples []float32) {
	t.Helper()
	if err := f.Write(samples); err != nil {
		t.Fatal(err)
	}
}

func pagesOf(t *testing.T, path string) []oggread.Page {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return oggread.Pages(data)
}

func TestEachStageGetsTheFrameSizeItNeeds(t *testing.T) {
	t.Parallel()

	enc, den := &fakeEncoder{}, &fakeDenoiser{}
	files := audiofile.NewFiles(
		func() (audiofile.Encoder, error) { return enc, nil },
		func() (audiofile.Denoiser, error) { return den, nil },
	)
	f, _ := create(t, files, true)

	for _, n := range []int{7, 1000, 333, 4800} {
		write(t, f, make([]float32, n))
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	if slices.ContainsFunc(den.frames, func(n int) bool { return n != 480 }) {
		t.Errorf("denoiser frames %v, want every one 480 samples", den.frames)
	}
	// 6140 samples plus the pre-skip of zeros, rounded up to whole 960-sample frames.
	if slices.ContainsFunc(enc.frames, func(n int) bool { return n != frame }) || len(enc.frames) != 7 {
		t.Errorf("encoder frames %v, want 7 frames of 960", enc.frames)
	}
}

func TestNoiseCleanupOffNeverBuildsTheDenoiser(t *testing.T) {
	t.Parallel()

	built := 0
	files := audiofile.NewFiles(
		func() (audiofile.Encoder, error) { return &fakeEncoder{}, nil },
		func() (audiofile.Denoiser, error) {
			built++
			return &fakeDenoiser{}, nil
		},
	)
	f, _ := create(t, files, false)
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	if built != 0 {
		t.Errorf("denoiser built %d times, want 0 with noise cleanup off", built)
	}
}

func TestCloseClosesTheEncoder(t *testing.T) {
	t.Parallel()

	enc := &fakeEncoder{}
	files := audiofile.NewFiles(
		func() (audiofile.Encoder, error) { return enc, nil },
		func() (audiofile.Denoiser, error) { return &fakeDenoiser{}, nil },
	)
	f, _ := create(t, files, true)

	if err := f.Close(); err != nil || enc.closes != 1 {
		t.Errorf("close %v, encoder closes %d: want the encoder released once", err, enc.closes)
	}
}

func TestASecondOfAudioIsOnDiskBeforeTheFileIsClosed(t *testing.T) {
	t.Parallel()

	f, path := create(t, realFiles(), false)
	t.Cleanup(func() { _ = f.Close() })
	write(t, f, tone(1.02))

	pages := pagesOf(t, path)

	if len(pages) < 3 || pages[2].Granule != 50*frame {
		t.Errorf("got %d pages on disk, want an audio page with granule 48000 before Close", len(pages))
	}
}

func TestCloseRecordsTheExactLength(t *testing.T) {
	t.Parallel()

	f, path := create(t, realFiles(), false)
	write(t, f, tone(1.234))
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	pages := pagesOf(t, path)
	last := pages[len(pages)-1]
	decoded := 0
	for _, p := range pages[2:] {
		decoded += len(p.Packets) * frame
	}
	if want := uint64(realPre + 59232); last.Flags&flagEOS == 0 || last.Granule != want {
		t.Errorf("last page flags %#x granule %d, want end of stream at %d (pre-skip + samples)",
			last.Flags, last.Granule, want)
	}
	if uint64(decoded) < last.Granule {
		t.Errorf("packets decode to %d samples, fewer than the final granule %d: the end was cut",
			decoded, last.Granule)
	}
}

func TestTheEndOfStreamPageCarriesTheLastAudio(t *testing.T) {
	t.Parallel()

	// 47,140 samples plus the padding fill exactly one 50-packet page, the case where the end could land on a page
	// of its own with nothing on it.
	f, path := create(t, realFiles(), false)
	write(t, f, tone(47140.0/48000))
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	pages := pagesOf(t, path)
	last := pages[len(pages)-1]
	if last.Flags&flagEOS == 0 || len(last.Packets) == 0 {
		t.Errorf("last page flags %#x with %d packets: the end-of-stream page must carry audio, or players "+
			"cannot apply its end trimming", last.Flags, len(last.Packets))
	}
	for i := 3; i < len(pages); i++ {
		if pages[i].Granule < pages[i-1].Granule {
			t.Errorf("page %d granule %d is below page %d's %d", i, pages[i].Granule, i-1, pages[i-1].Granule)
		}
	}
}

func TestAFileCutShortStillDecodesUpToTheCut(t *testing.T) {
	t.Parallel()

	f, path := create(t, realFiles(), false)
	write(t, f, tone(5))
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var packets [][]byte
	for _, p := range oggread.Pages(data[:len(data)-100])[2:] {
		packets = append(packets, p.Packets...)
	}
	pcm, err := opustest.Decode(packets)
	if err != nil {
		t.Fatal(err)
	}

	var sum float64
	for _, v := range pcm {
		x := float64(v) / 32768
		sum += x * x
	}
	if len(pcm) < 4*sampleRate || 10*math.Log10(sum/float64(len(pcm))) < -30 {
		t.Errorf("decoded %d samples, want at least 4 s of the tone from a file cut short", len(pcm))
	}
}

type failingSink struct {
	failing bool
}

func (s *failingSink) Write(p []byte) (int, error) {
	if s.failing {
		return 0, audiofile.ErrDiskFullFromOS
	}
	return len(p), nil
}

func (s *failingSink) Sync() error { return nil }

func (s *failingSink) Close() error { return nil }

func TestDiskFullBecomesTheApplicationsError(t *testing.T) {
	t.Parallel()

	sink := &failingSink{}
	f, err := audiofile.NewFileOn(sink, "rec.ogg", &fakeEncoder{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	sink.failing = true

	err = f.Write(tone(2))

	if !errors.Is(err, application.ErrDiskFull) {
		t.Errorf("err = %v, want ErrDiskFull so the recorder can tell the owner", err)
	}
}
