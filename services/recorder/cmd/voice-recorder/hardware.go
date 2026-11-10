package main

import (
	"context"

	"github.com/tienduc120/voice-recorder/services/recorder/internal/application"
)

type sleepGuard interface {
	application.SleepGuard
	Run(ctx context.Context) error
}

// hardware is everything that touches the machine: the mics, the dialog, the browser, sleep. Tests swap it.
type hardware struct {
	mics    application.Microphones
	capture application.Capture
	picker  application.FolderPicker
	awake   sleepGuard
	// captureRun is the capture adapter's loop: it owns the COM thread the mic streams live on.
	captureRun   func(ctx context.Context) error
	openBrowser  func(url string) error
	alert        func(title, text string)
	documentsDir string
}
