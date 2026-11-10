package winsys

import (
	"context"
	"runtime"

	"golang.org/x/sys/windows"
)

const (
	esContinuous     = 0x80000000
	esSystemRequired = 0x00000001
)

// SleepGuard asks Windows not to sleep while a recording is open. The screen may still turn off.
type SleepGuard struct {
	set func(flags uint32)
	// requests holds the latest Hold or Release; an older one still waiting is replaced.
	requests chan bool
}

func NewSleepGuard() *SleepGuard {
	proc := windows.NewLazySystemDLL("kernel32.dll").NewProc("SetThreadExecutionState")
	return newSleepGuard(func(flags uint32) { _, _, _ = proc.Call(uintptr(flags)) })
}

func newSleepGuard(set func(flags uint32)) *SleepGuard {
	return &SleepGuard{set: set, requests: make(chan bool, 1)}
}

func (g *SleepGuard) Hold() { g.request(true) }

func (g *SleepGuard) Release() { g.request(false) }

func (g *SleepGuard) request(hold bool) {
	for {
		select {
		case g.requests <- hold:
			return
		default:
			select {
			case <-g.requests:
			default:
			}
		}
	}
}

// Run owns one locked OS thread until ctx ends, then lets Windows sleep again. SetThreadExecutionState belongs to
// the thread that calls it, and goroutines move between threads, so every call is made here.
func (g *SleepGuard) Run(ctx context.Context) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer g.set(esContinuous)
	for {
		select {
		case <-ctx.Done():
			return nil
		case hold := <-g.requests:
			if hold {
				g.set(esContinuous | esSystemRequired)
			} else {
				g.set(esContinuous)
			}
		}
	}
}
