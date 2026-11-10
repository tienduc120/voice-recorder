package application

import (
	"time"

	"github.com/tienduc120/voice-recorder/services/recorder/internal/domain"
)

type State string

const (
	StateIdle          State = "idle"
	StateRecording     State = "recording"
	StatePaused        State = "paused"
	StateWaitingForMic State = "waiting-for-mic"
)

// NoticeKind names an event for the status line; the page owns the words, so the kind is all that travels.
type NoticeKind string

const (
	NoticeMicConnected NoticeKind = "mic-connected"
	NoticeMicFallback  NoticeKind = "mic-fallback"
	NoticeNoMic        NoticeKind = "no-mic"
	NoticeDiskFull     NoticeKind = "disk-full"
	NoticeWriteFailed  NoticeKind = "write-failed"
)

type Notice struct {
	Kind        NoticeKind
	Mic         string
	PreviousMic string
}

type FileInfo struct {
	Name string
	Size int64
}

type Status struct {
	State    State
	Recorded time.Duration
	// Level is the input level from 0 to 1, and 0 when idle.
	Level         float64
	Folder        string
	FolderProblem string
	NoiseCleanup  bool
	Mics          []Device
	// MicID is the mic in use while a recording is active, and the remembered one while idle.
	MicID  string
	File   FileInfo
	Notice Notice
}

func stateOf(s domain.State) State {
	switch s {
	case domain.Idle:
		return StateIdle
	case domain.Recording:
		return StateRecording
	case domain.Paused:
		return StatePaused
	case domain.WaitingForMic:
		return StateWaitingForMic
	}
	return StateIdle
}
