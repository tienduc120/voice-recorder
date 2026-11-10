//go:build windows

package winsys

const (
	ESContinuous     = esContinuous
	ESSystemRequired = esSystemRequired
)

// NewSleepGuardWith replaces SetThreadExecutionState, so a test can see each call and the thread it ran on.
func NewSleepGuardWith(set func(flags uint32)) *SleepGuard { return newSleepGuard(set) }
