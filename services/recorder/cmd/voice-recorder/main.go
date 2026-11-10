// Command voice-recorder records meetings from a mic into small Ogg/Opus files, controlled from a local web page.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"time"
)

const (
	// appAddr is fixed because the port is also how a second launch finds the running copy; loopback keeps every
	// other computer out.
	appAddr        = "127.0.0.1:47811"
	appDirName     = "VoiceRecorder"
	settingsName   = "config.json"
	logName        = "voice-recorder.log"
	recordingsName = "Recordings"
	alertTitle     = "Voice Recorder"
	handOffTimeout = 3 * time.Second
	// apiWriteTimeout is long because the folder dialog holds its request open while the owner picks.
	apiWriteTimeout = 10 * time.Minute

	exitOK   = 0
	exitFail = 1
)

type config struct {
	addr      string
	debugAddr string
	appDir    string
}

// configFrom sets the debug listener only when DEBUG_ADDR asks for it: profiling is for checking memory, not
// for every run.
func configFrom(getenv func(string) string, userConfigDir string) config {
	return config{
		addr:      appAddr,
		debugAddr: getenv("DEBUG_ADDR"),
		appDir:    filepath.Join(userConfigDir, appDirName),
	}
}

func main() { os.Exit(runMain()) }

func runMain() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	configDir, err := os.UserConfigDir()
	if err != nil {
		platformAlert(alertTitle, fmt.Sprintf("Voice Recorder could not find its settings folder: %v", err))
		return exitFail
	}
	cfg := configFrom(os.Getenv, configDir)

	// The GUI build has no console, so the log file is where a problem can be read afterwards.
	logger := slog.New(slog.DiscardHandler)
	if file, logErr := openLog(cfg.appDir); logErr == nil {
		defer func() { _ = file.Close() }()
		logger = slog.New(slog.NewTextHandler(file, nil))
	}

	hw, err := newHardware(logger)
	if err != nil {
		platformAlert(alertTitle, fmt.Sprintf("Voice Recorder could not start: %v", err))
		return exitFail
	}
	return launch(ctx, logger, cfg, hw)
}

// launch starts the app, or opens the copy that is already running: there is never a second recorder.
func launch(ctx context.Context, logger *slog.Logger, cfg config, hw hardware) int {
	a, err := newApp(ctx, logger, cfg, hw)
	if err != nil {
		if handOff(ctx, cfg.addr) {
			logger.InfoContext(ctx, "already running, opening its page", "addr", cfg.addr)
			openPage(ctx, logger, hw, cfg.addr)
			return exitOK
		}
		logger.ErrorContext(ctx, "start", "error", err)
		hw.alert(alertTitle, fmt.Sprintf("Voice Recorder could not start on %s:\n%v", cfg.addr, err))
		return exitFail
	}
	logger.InfoContext(ctx, "listening", "addr", a.api.Addr())
	openPage(ctx, logger, hw, a.api.Addr())
	if err = a.run(ctx); err != nil {
		logger.ErrorContext(ctx, "stopped", "error", err)
		hw.alert(alertTitle, fmt.Sprintf("Voice Recorder stopped: %v", err))
		return exitFail
	}
	return exitOK
}

func openPage(ctx context.Context, logger *slog.Logger, hw hardware, addr string) {
	if err := hw.openBrowser("http://" + addr + "/"); err != nil {
		logger.WarnContext(ctx, "open the browser", "error", err)
	}
}
