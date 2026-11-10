//go:build windows

package winsys_test

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"golang.org/x/sys/windows"

	"github.com/tienduc120/voice-recorder/services/recorder/internal/infrastructure/winsys"
)

type call struct {
	flags  uint32
	thread uint32
}

func TestSleepGuardSetsAndClearsTheStateOnOneThread(t *testing.T) {
	t.Parallel()

	calls := make(chan call, 4)
	g := winsys.NewSleepGuardWith(func(flags uint32) {
		calls <- call{flags: flags, thread: windows.GetCurrentThreadId()}
	})
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- g.Run(ctx) }()

	g.Hold()
	held := <-calls
	g.Release()
	released := <-calls
	cancel()
	err := <-done
	cleared := <-calls

	want := []uint32{winsys.ESContinuous | winsys.ESSystemRequired, winsys.ESContinuous, winsys.ESContinuous}
	if got := []uint32{held.flags, released.flags, cleared.flags}; !reflect.DeepEqual(got, want) {
		t.Errorf("flags %#x, want %#x: hold, release, and a release on the way out", got, want)
	}
	if held.thread != released.thread || released.thread != cleared.thread {
		t.Errorf("threads %d, %d, %d: SetThreadExecutionState belongs to one thread, so every call must use it",
			held.thread, released.thread, cleared.thread)
	}
	if err != nil {
		t.Errorf("run = %v, want nil when told to stop", err)
	}
}

func TestDocumentsDirIsAnExistingAbsoluteFolder(t *testing.T) {
	t.Parallel()

	dir, err := winsys.DocumentsDir()
	if err != nil {
		t.Fatal(err)
	}

	info, statErr := os.Stat(dir)
	if !filepath.IsAbs(dir) || statErr != nil || !info.IsDir() {
		t.Errorf("documents dir %q (%v): want an existing absolute folder", dir, statErr)
	}
}
