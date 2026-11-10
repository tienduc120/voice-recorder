//go:build !windows

package main

import (
	"errors"
	"log/slog"
)

var errWindowsOnly = errors.New("voice-recorder runs on Windows only")

func newHardware(*slog.Logger) (hardware, error) { return hardware{}, errWindowsOnly }

func platformAlert(string, string) {}
