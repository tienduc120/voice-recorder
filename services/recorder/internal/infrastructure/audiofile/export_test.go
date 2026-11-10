package audiofile

import "github.com/tienduc120/voice-recorder/services/recorder/internal/application"

// ErrDiskFullFromOS is the platform's "disk full" error, for a sink that fails like a full disk.
const ErrDiskFullFromOS = errDiskFull

// FileSink is what NewFileOn writes to.
type FileSink = sink

// NewFileOn builds a recording on any sink, so a test can make the disk fail.
func NewFileOn(out FileSink, name string, enc Encoder, den Denoiser) (application.AudioFile, error) {
	return newFile(out, name, enc, den)
}
