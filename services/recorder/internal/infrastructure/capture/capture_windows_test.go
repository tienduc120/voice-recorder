//go:build windows

package capture_test

import (
	"context"
	"log/slog"
	"testing"
	"testing/synctest"
	"time"

	"github.com/moutend/go-wca/pkg/wca"

	"github.com/tienduc120/voice-recorder/services/recorder/internal/application"
	"github.com/tienduc120/voice-recorder/services/recorder/internal/infrastructure/capture"
)

const blockSize = 480

func quiet() *slog.Logger { return slog.New(slog.DiscardHandler) }

// running starts the capture loop for the rest of the test.
func running(t *testing.T) *capture.Capture {
	t.Helper()
	c := capture.New(quiet())
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- c.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Errorf("run = %v, want nil when told to stop", err)
		}
	})
	return c
}

func TestStreamFormatAsksForFloat48kMonoWithConversion(t *testing.T) {
	t.Parallel()

	format, flags := capture.StreamFormat()

	want := wca.WAVEFORMATEX{
		WFormatTag: 3, NChannels: 1, NSamplesPerSec: 48000, NAvgBytesPerSec: 192000, NBlockAlign: 4, WBitsPerSample: 32,
	}
	if format != want {
		t.Errorf("format %+v, want %+v: Windows converts any mic to 48 kHz mono float", format, want)
	}
	for name, flag := range map[string]uint32{
		"AUTOCONVERTPCM":      wca.AUDCLNT_STREAMFLAGS_AUTOCONVERTPCM,
		"SRC_DEFAULT_QUALITY": wca.AUDCLNT_STREAMFLAGS_SRC_DEFAULT_QUALITY,
		"EVENTCALLBACK":       wca.AUDCLNT_STREAMFLAGS_EVENTCALLBACK,
	} {
		if flags&flag == 0 {
			t.Errorf("flags %#x lack %s", flags, name)
		}
	}
}

func TestListMarksAtMostOneDefault(t *testing.T) {
	t.Parallel()

	devices, err := capture.New(quiet()).List(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	defaults := 0
	for _, d := range devices {
		if d.IsDefault {
			defaults++
		}
		if d.ID == "" || d.Name == "" {
			t.Errorf("device %+v: every mic has an ID and a name", d)
		}
	}
	if defaults > 1 {
		t.Errorf("%d mics marked default, want at most one", defaults)
	}
}

func TestOpenWithoutRunFailsInsteadOfHanging(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		_, err := capture.New(quiet()).Open("{any}", make(chan []float32, 1))

		if err == nil {
			t.Error("Open must fail when the capture loop is not running")
		}
	})
}

func TestOpeningAnUnknownIDFails(t *testing.T) {
	t.Parallel()

	_, err := running(t).Open("{not-a-device}", make(chan []float32, 1))

	if err == nil {
		t.Error("opening an ID no mic has must fail")
	}
}

func TestCaptureDeliversBlocksFromTheDefaultMic(t *testing.T) {
	t.Parallel()

	c := running(t)
	devices, err := c.List(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	id := defaultID(devices)
	if id == "" {
		t.Skip("no capture device: plug in a mic to run this test")
	}
	frames := make(chan []float32, 400)

	stream, err := c.Open(id, frames)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case block := <-frames:
		if len(block) != blockSize {
			t.Errorf("block of %d samples, want %d", len(block), blockSize)
		}
	case <-time.After(5 * time.Second):
		t.Error("no audio within 5 s of opening the mic")
	}

	if err = stream.Close(); err != nil {
		t.Errorf("close = %v", err)
	}
	select {
	case <-stream.Lost():
		t.Error("a normal Close must not report the mic as lost")
	default:
	}
}

func defaultID(devices []application.Device) string {
	for _, d := range devices {
		if d.IsDefault {
			return d.ID
		}
	}
	if len(devices) > 0 {
		return devices[0].ID
	}
	return ""
}
