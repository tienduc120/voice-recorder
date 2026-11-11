//go:build windows

package capture

import (
	"testing"

	"github.com/moutend/go-wca/pkg/wca"
)

func TestAMicWhoseNameIsNotAStringIsCalledMicrophone(t *testing.T) {
	t.Parallel()

	if got := nameFrom(&wca.PROPVARIANT{}); got != unnamed {
		t.Errorf("name of an empty property = %q, want %q", got, unnamed)
	}
}

func TestAFailedGetIDIsAnErrorWhoseResultIsNeverRead(t *testing.T) {
	t.Parallel()

	const elementNotFound = 0x80070490
	if _, err := takeID(elementNotFound, nil); err == nil {
		t.Error("a failed GetId gave no error")
	}
	if _, err := takeID(0, nil); err == nil {
		t.Error("a GetId that returned no string gave no error")
	}
}
