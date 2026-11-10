package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/tienduc120/voice-recorder/services/recorder/internal/interfaces/rest"
)

const (
	maxPingBytes = 1 << 10
	logMode      = 0o600
	logDirMode   = 0o750
	previousLog  = ".1"
)

// handOff reports whether the program holding addr is a running Voice Recorder, which then gets the page instead
// of a second recorder.
func handOff(ctx context.Context, addr string) bool {
	ctx, cancel := context.WithTimeout(ctx, handOffTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+addr+"/api/ping", nil)
	if err != nil {
		return false
	}
	client := &http.Client{Timeout: handOffTimeout, Transport: &http.Transport{DisableKeepAlives: true}}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()
	var ping struct {
		App string `json:"app"`
	}
	if resp.StatusCode != http.StatusOK ||
		json.NewDecoder(io.LimitReader(resp.Body, maxPingBytes)).Decode(&ping) != nil {
		return false
	}
	return ping.App == rest.AppName
}

// openLog keeps the previous run's log as .1. It appends rather than truncates: a second launch that cannot
// rename the running copy's log must not wipe it.
func openLog(dir string) (*os.File, error) {
	if err := os.MkdirAll(dir, logDirMode); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, logName)
	// A failed rename means the running copy holds the log open; this launch then appends to it.
	_ = os.Rename(path, path+previousLog)
	return os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, logMode)
}
