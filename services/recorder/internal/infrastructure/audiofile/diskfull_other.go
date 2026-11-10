//go:build !windows

package audiofile

import (
	"errors"
	"syscall"
)

const errDiskFull = syscall.ENOSPC

func isDiskFull(err error) bool { return errors.Is(err, errDiskFull) }
