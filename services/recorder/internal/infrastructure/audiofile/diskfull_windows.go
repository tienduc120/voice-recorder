package audiofile

import (
	"errors"
	"syscall"
)

const (
	// errDiskFull is ERROR_DISK_FULL; errHandleDiskFull is ERROR_HANDLE_DISK_FULL, which some writes report instead.
	errDiskFull       = syscall.Errno(112)
	errHandleDiskFull = syscall.Errno(39)
)

func isDiskFull(err error) bool {
	return errors.Is(err, errDiskFull) || errors.Is(err, errHandleDiskFull)
}
