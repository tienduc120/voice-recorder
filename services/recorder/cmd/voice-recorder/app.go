package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/tienduc120/voice-recorder/platform/async"
	"github.com/tienduc120/voice-recorder/platform/clock"
	"github.com/tienduc120/voice-recorder/platform/debugserver"
	"github.com/tienduc120/voice-recorder/platform/httpserver"
	"github.com/tienduc120/voice-recorder/services/recorder/internal/application"
	"github.com/tienduc120/voice-recorder/services/recorder/internal/infrastructure/audiofile"
	"github.com/tienduc120/voice-recorder/services/recorder/internal/infrastructure/denoise"
	"github.com/tienduc120/voice-recorder/services/recorder/internal/infrastructure/opusenc"
	"github.com/tienduc120/voice-recorder/services/recorder/internal/infrastructure/settingsfile"
	"github.com/tienduc120/voice-recorder/services/recorder/internal/interfaces/rest"
)

const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 15 * time.Second
	idleTimeout       = time.Minute
	shutdownTimeout   = 10 * time.Second
	folderMode        = 0o750
)

// app owns the listeners and the recorder. The listeners are bound before anything runs, so a taken port fails
// the launch, which is how a second copy notices the first.
type app struct {
	api      *httpserver.Server
	debug    *debugserver.Server
	recorder *application.Recorder
	hw       hardware
	quit     chan struct{}
	quitOnce sync.Once
}

func newApp(ctx context.Context, logger *slog.Logger, cfg config, hw hardware) (*app, error) {
	defaults := application.Settings{Folder: filepath.Join(hw.documentsDir, recordingsName), NoiseCleanup: true}
	store := settingsfile.New(filepath.Join(cfg.appDir, settingsName), defaults)
	ensureDefaultFolder(ctx, logger, store, defaults.Folder)

	a := &app{hw: hw, quit: make(chan struct{})}
	a.recorder = application.NewRecorder(ctx, application.Deps{
		Clock:    clock.System{},
		Mics:     hw.mics,
		Capture:  hw.capture,
		Files:    audiofile.NewFiles(newEncoder, newDenoiser),
		Settings: store,
		Picker:   hw.picker,
		Awake:    hw.awake,
		Logger:   logger,
		Quit:     a.requestQuit,
	})

	api, err := httpserver.New(ctx, cfg.addr, rest.NewHandler(a.recorder, logger), httpserver.Timeouts{
		ReadHeader: readHeaderTimeout,
		Read:       readTimeout,
		Write:      apiWriteTimeout,
		Idle:       idleTimeout,
		Shutdown:   shutdownTimeout,
	})
	if err != nil {
		return nil, fmt.Errorf("control page listener: %w", err)
	}
	a.api = api
	if cfg.debugAddr != "" {
		debug, debugErr := debugserver.New(ctx, cfg.debugAddr)
		if debugErr != nil {
			_ = api.Close()
			return nil, fmt.Errorf("debug listener: %w", debugErr)
		}
		debugserver.EnableContentionProfiles()
		a.debug = debug
	}
	return a, nil
}

// run serves until ctx ends, Quit is asked, or a task fails; any of them stops the rest, and the recorder
// finalises an open recording on its way out.
func (a *app) run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	tasks := []func(context.Context) error{
		a.api.Run,
		a.recorder.Run,
		a.hw.awake.Run,
		a.hw.captureRun,
		func(ctx context.Context) error {
			select {
			case <-a.quit:
				cancel()
			case <-ctx.Done():
			}
			return nil
		},
	}
	if a.debug != nil {
		tasks = append(tasks, a.debug.Run)
	}
	return async.Run(ctx, tasks...)
}

func (a *app) requestQuit() { a.quitOnce.Do(func() { close(a.quit) }) }

// ensureDefaultFolder creates Documents\Recordings on the first run; a folder the owner chose is never created.
func ensureDefaultFolder(ctx context.Context, logger *slog.Logger, store *settingsfile.Store, defaultFolder string) {
	settings, err := store.Load()
	if err != nil {
		logger.WarnContext(ctx, "settings unreadable, using defaults", "error", err)
	}
	if settings.Folder != defaultFolder {
		return
	}
	if err = os.MkdirAll(defaultFolder, folderMode); err != nil {
		logger.WarnContext(ctx, "create the default folder", "folder", defaultFolder, "error", err)
	}
}

func newEncoder() (audiofile.Encoder, error) {
	e, err := opusenc.New()
	if err != nil {
		return nil, err
	}
	return e, nil
}

func newDenoiser() (audiofile.Denoiser, error) {
	d, err := denoise.New()
	if err != nil {
		return nil, err
	}
	return d, nil
}
