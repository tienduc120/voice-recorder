package domain_test

import (
	"reflect"
	"testing"

	"github.com/tienduc120/voice-recorder/services/recorder/internal/domain"
)

const (
	builtInName = "Microphone Array (Realtek)"
	usbName     = "USB Microphone"
)

func assertChoice(t *testing.T, c domain.Choice, wantID domain.MicID, wantReason domain.Reason) {
	t.Helper()
	mic, ok := c.Mic()
	if !ok || mic.ID() != wantID || c.Reason() != wantReason {
		t.Errorf("chose %q (ok %v) for reason %v, want %q for reason %v", mic.ID(), ok, c.Reason(), wantID, wantReason)
	}
}

func TestPluggingInAMicNeverChangesTheOneInUse(t *testing.T) {
	t.Parallel()

	builtIn := domain.NewMic("builtin", builtInName, false)
	usbNowDefault := domain.NewMic("usb", usbName, true)

	c := domain.ChooseMic([]domain.Mic{builtIn, usbNowDefault}, "builtin", "builtin")

	assertChoice(t, c, "builtin", domain.KeptCurrent)
}

func TestAtStartTheLastUsedMicWins(t *testing.T) {
	t.Parallel()

	builtInDefault := domain.NewMic("builtin", builtInName, true)
	usb := domain.NewMic("usb", usbName, false)

	c := domain.ChooseMic([]domain.Mic{builtInDefault, usb}, "", "usb")

	assertChoice(t, c, "usb", domain.UsedPreferred)
}

func TestAMissingMicFallsBackToTheWindowsDefault(t *testing.T) {
	t.Parallel()

	builtInDefault := domain.NewMic("builtin", builtInName, true)

	c := domain.ChooseMic([]domain.Mic{builtInDefault}, "usb", "usb")

	assertChoice(t, c, "builtin", domain.FellBackToDefault)
}

func TestWithNoDefaultMarkedTheFirstMicIsUsed(t *testing.T) {
	t.Parallel()

	a := domain.NewMic("a", "A", false)
	b := domain.NewMic("b", "B", false)

	c := domain.ChooseMic([]domain.Mic{a, b}, "", "gone")

	assertChoice(t, c, "a", domain.FellBackToDefault)
}

func TestNoDevicesMeansNoMic(t *testing.T) {
	t.Parallel()

	c := domain.ChooseMic(nil, "builtin", "builtin")

	if _, ok := c.Mic(); ok || c.Reason() != domain.NoMic {
		t.Errorf("ok %v, reason %v: with no devices there is no mic to choose", ok, c.Reason())
	}
}

func TestNewlyConnectedComparesByID(t *testing.T) {
	t.Parallel()

	builtIn := domain.NewMic("builtin", builtInName, true)
	renamedBuiltIn := domain.NewMic("builtin", "Microphone (renamed)", true)
	usb := domain.NewMic("usb", usbName, false)

	got := domain.NewlyConnected([]domain.Mic{builtIn}, []domain.Mic{renamedBuiltIn, usb})

	if want := []domain.Mic{usb}; !reflect.DeepEqual(got, want) {
		t.Errorf("newly connected = %+v, want %+v: a renamed mic is not a new one", got, want)
	}
}
