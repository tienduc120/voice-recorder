package application

import "errors"

var (
	ErrInvalidCommand = errors.New("invalid command")
	// ErrConflict means the recorder's state does not allow the command now, such as a second Start.
	ErrConflict          = errors.New("not possible in the current state")
	ErrFolderNotWritable = errors.New("folder is not writable")
	ErrDiskFull          = errors.New("disk is full")
)
