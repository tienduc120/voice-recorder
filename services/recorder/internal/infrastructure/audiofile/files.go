// Package audiofile writes a recording, the signal stages, the Opus encoder and the Ogg muxer, into one file.
package audiofile

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tienduc120/voice-recorder/services/recorder/internal/application"
)

const (
	// maxNameAttempts bounds the -2, -3, … suffixes; a thousand recordings in the same minute is not a meeting.
	maxNameAttempts = 999
	// fileMode keeps recordings private to the owner, as gosec asks.
	fileMode    = 0o600
	probePrefix = ".voice-recorder-*"
)

type Encoder interface {
	Encode(pcm []float32, out []byte) (int, error)
	PreSkip() int
	Vendor() string
	Close() error
}

type Denoiser interface {
	Process(frame []float32) error
}

// Files makes recordings. The encoder and denoiser are made per recording, because each keeps the state of one
// stream.
type Files struct {
	newEncoder  func() (Encoder, error)
	newDenoiser func() (Denoiser, error)
}

func NewFiles(newEncoder func() (Encoder, error), newDenoiser func() (Denoiser, error)) *Files {
	return &Files{newEncoder: newEncoder, newDenoiser: newDenoiser}
}

// CheckFolder proves a recording could be saved in folder by creating and removing a probe file there.
func (f *Files) CheckFolder(folder string) error {
	if !filepath.IsAbs(folder) {
		return fmt.Errorf(
			"%w: %q is not a full path; use one such as C:\\Recordings",
			application.ErrFolderNotWritable,
			folder,
		)
	}
	info, err := os.Stat(folder)
	if err != nil {
		return fmt.Errorf("%w: %w", application.ErrFolderNotWritable, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%w: %q is a file, not a folder", application.ErrFolderNotWritable, folder)
	}
	probe, err := os.CreateTemp(folder, probePrefix)
	if err != nil {
		return fmt.Errorf("%w: %w", application.ErrFolderNotWritable, err)
	}
	path := probe.Name()
	return errors.Join(closeProbe(probe), removeProbe(path))
}

func closeProbe(probe *os.File) error {
	if err := probe.Close(); err != nil {
		return fmt.Errorf("%w: %w", application.ErrFolderNotWritable, err)
	}
	return nil
}

func removeProbe(path string) error {
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("%w: %w", application.ErrFolderNotWritable, err)
	}
	return nil
}

// Create opens the first free name and writes the Ogg Opus headers, so the file already plays if the app dies.
func (f *Files) Create(
	folder string,
	name func(attempt int) string,
	opts application.AudioOptions,
) (application.AudioFile, error) {
	out, base, err := createFree(folder, name)
	if err != nil {
		return nil, err
	}
	enc, den, err := f.stages(opts)
	if err != nil {
		return nil, errors.Join(err, out.Close(), os.Remove(out.Name()))
	}
	file, err := newFile(out, base, enc, den)
	if err != nil {
		return nil, errors.Join(err, enc.Close(), out.Close(), os.Remove(out.Name()))
	}
	return file, nil
}

func (f *Files) stages(opts application.AudioOptions) (Encoder, Denoiser, error) {
	enc, err := f.newEncoder()
	if err != nil {
		return nil, nil, fmt.Errorf("encoder: %w", err)
	}
	if !opts.NoiseCleanup {
		return enc, nil, nil
	}
	den, err := f.newDenoiser()
	if err != nil {
		return nil, nil, errors.Join(fmt.Errorf("noise cleanup: %w", err), enc.Close())
	}
	return enc, den, nil
}

// createFree claims a name with O_EXCL, so two recordings can never share a file even if they start together.
func createFree(folder string, name func(attempt int) string) (*os.File, string, error) {
	for attempt := 1; attempt <= maxNameAttempts; attempt++ {
		base := name(attempt)
		out, err := os.OpenFile(filepath.Join(folder, base), os.O_WRONLY|os.O_CREATE|os.O_EXCL, fileMode)
		if err == nil {
			return out, base, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, "", createError(err)
		}
	}
	return nil, "", fmt.Errorf(
		"%w: names up to attempt %d are all taken",
		application.ErrFolderNotWritable,
		maxNameAttempts,
	)
}

func createError(err error) error {
	switch {
	case isDiskFull(err):
		return fmt.Errorf("create the recording: %w: %w", application.ErrDiskFull, err)
	case errors.Is(err, os.ErrPermission), errors.Is(err, os.ErrNotExist):
		return fmt.Errorf("create the recording: %w: %w", application.ErrFolderNotWritable, err)
	default:
		return fmt.Errorf("create the recording: %w", err)
	}
}

// writeError lets the recorder tell a full disk from any other failure.
func writeError(err error) error {
	if isDiskFull(err) {
		return fmt.Errorf("%w: %w", application.ErrDiskFull, err)
	}
	return err
}

// serial is the Ogg stream's serial number; random so files concatenated by some tool stay separable.
func serial() uint32 {
	var b [4]byte
	// crypto/rand.Read never returns an error; it ends the program instead.
	_, _ = rand.Read(b[:])
	return binary.LittleEndian.Uint32(b[:])
}
