package winsys

import (
	"context"
	"errors"
	"fmt"
	"os"
)

var ErrNotAFolder = errors.New("not a folder")

// FolderOpener shows a folder in File Explorer. The zero value is ready to use.
type FolderOpener struct {
	open func(path string) error
}

func (o FolderOpener) Open(_ context.Context, folder string) error {
	info, err := os.Stat(folder)
	if err != nil {
		return fmt.Errorf("open the folder: %w", err)
	}
	// ShellExecute runs any program it is handed, so only a folder may reach it.
	if !info.IsDir() {
		return fmt.Errorf("open the folder: %q: %w", folder, ErrNotAFolder)
	}
	open := o.open
	if open == nil {
		open = shellOpen
	}
	if err = open(folder); err != nil {
		return fmt.Errorf("open the folder: %w", err)
	}
	return nil
}
