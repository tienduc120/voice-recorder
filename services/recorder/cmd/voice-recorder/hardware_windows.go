package main

import (
	"log/slog"
	"os"
	"path/filepath"

	"github.com/tienduc120/voice-recorder/services/recorder/internal/infrastructure/capture"
	"github.com/tienduc120/voice-recorder/services/recorder/internal/infrastructure/winsys"
)

func newHardware(logger *slog.Logger) (hardware, error) {
	docs, err := winsys.DocumentsDir()
	if err != nil {
		// Only if Windows cannot name the Documents folder: the spec's original default.
		home, homeErr := os.UserHomeDir()
		if homeErr != nil {
			return hardware{}, homeErr
		}
		docs = filepath.Join(home, "Documents")
	}
	mics := capture.New(logger)
	return hardware{
		mics:         mics,
		capture:      mics,
		picker:       winsys.FolderPicker{},
		opener:       winsys.FolderOpener{},
		awake:        winsys.NewSleepGuard(),
		captureRun:   mics.Run,
		openBrowser:  winsys.OpenBrowser,
		alert:        winsys.Alert,
		documentsDir: docs,
	}, nil
}

func platformAlert(title, text string) { winsys.Alert(title, text) }
