package application

import (
	"context"
	"time"
)

type Clock interface {
	Now() time.Time
}

type Device struct {
	ID        string
	Name      string
	IsDefault bool
}

type Microphones interface {
	List(ctx context.Context) ([]Device, error)
}

// Capture sends a mic as blocks of 480 float32 samples (10 ms of 48 kHz mono) on frames, and never blocks on a full
// channel: a slow consumer loses audio, it never stalls the device.
type Capture interface {
	Open(deviceID string, frames chan<- []float32) (Stream, error)
}

type Stream interface {
	// Lost is closed when the device stops by itself (unplugged, reset); a Close never closes it.
	Lost() <-chan struct{}
	Close() error
}

type AudioOptions struct {
	NoiseCleanup bool
}

type AudioFiles interface {
	// CheckFolder wraps ErrFolderNotWritable when a recording could not be saved there.
	CheckFolder(folder string) error
	// Create tries name(1), name(2), … in folder until a name is free.
	Create(folder string, name func(attempt int) string, opts AudioOptions) (AudioFile, error)
}

type AudioFile interface {
	Name() string
	// Size is the bytes written so far.
	Size() int64
	// Write takes 48 kHz mono samples of any length and wraps ErrDiskFull when the disk is full.
	Write(samples []float32) error
	// Flush puts the buffered audio on disk now.
	Flush() error
	Close() error
}

type Settings struct {
	Folder       string
	MicID        string
	NoiseCleanup bool
}

// SettingsStore keeps the settings between launches. Load always returns usable settings: the store's defaults,
// with an error, when the stored ones cannot be read.
type SettingsStore interface {
	Load() (Settings, error)
	Save(settings Settings) error
}

type FolderPicker interface {
	Pick(ctx context.Context, start string) (folder string, chosen bool, err error)
}

// FolderOpener shows a folder in File Explorer.
type FolderOpener interface {
	Open(ctx context.Context, folder string) error
}

// SleepGuard keeps Windows from sleeping while held.
type SleepGuard interface {
	Hold()
	Release()
}
