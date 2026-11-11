// Package application runs the recorder's use cases and declares the ports its adapters implement.
package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/tienduc120/voice-recorder/services/recorder/internal/domain"
)

// frameBuffer holds 2 s of 10 ms blocks; past that the capture drops audio rather than growing memory.
const frameBuffer = 200

type Deps struct {
	Clock    Clock
	Mics     Microphones
	Capture  Capture
	Files    AudioFiles
	Settings SettingsStore
	Picker   FolderPicker
	Opener   FolderOpener
	Awake    SleepGuard
	Logger   *slog.Logger
	// Quit asks the app to exit once the use case has finished.
	Quit func()
}

type StartCommand struct {
	Title string
}

type SettingsCommand struct {
	Folder       string
	NoiseCleanup bool
}

type BrowseResult struct {
	Folder string
	Chosen bool
}

// Recorder owns the one recording session. One mutex guards it: the use cases and the frame loop are short, and a
// single lock keeps every state change in one order.
type Recorder struct {
	deps Deps
	// frames is shared by every stream, so switching mics never changes what the frame loop reads.
	frames chan []float32

	mu            sync.Mutex
	settings      Settings
	folderProblem string
	devices       []domain.Mic
	session       domain.Session
	file          AudioFile
	stream        Stream
	micID         domain.MicID
	lastFile      FileInfo
	level         float64
	notice        Notice
}

func NewRecorder(ctx context.Context, d Deps) *Recorder {
	r := &Recorder{deps: d, frames: make(chan []float32, frameBuffer)}
	settings, err := d.Settings.Load()
	if err != nil {
		d.Logger.WarnContext(ctx, "settings unreadable, using defaults", "error", err)
	}
	r.settings = settings
	r.folderProblem = r.folderCheck(settings.Folder)
	r.refreshDevices(ctx)
	return r
}

func (r *Recorder) Status(context.Context) Status {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.statusLocked()
}

func (r *Recorder) ChangeSettings(ctx context.Context, cmd SettingsCommand) (Status, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if st := r.session.State(); st != domain.Idle {
		return r.statusLocked(), fmt.Errorf("%w: settings change only while idle, not while %s", ErrConflict, st)
	}
	folder := cleanFolder(cmd.Folder)
	if err := r.deps.Files.CheckFolder(folder); err != nil {
		return r.statusLocked(), fmt.Errorf("%w: %w", ErrInvalidCommand, err)
	}
	r.settings.Folder = folder
	r.settings.NoiseCleanup = cmd.NoiseCleanup
	r.folderProblem = ""
	r.saveSettings(ctx)
	return r.statusLocked(), nil
}

// Browse never holds the lock: the dialog can stay open for minutes while the frame loop keeps recording.
func (r *Recorder) Browse(ctx context.Context) (BrowseResult, error) {
	r.mu.Lock()
	start := r.settings.Folder
	r.mu.Unlock()
	folder, chosen, err := r.deps.Picker.Pick(ctx, start)
	if err != nil {
		return BrowseResult{}, fmt.Errorf("folder picker: %w", err)
	}
	return BrowseResult{Folder: folder, Chosen: chosen}, nil
}

// OpenFolder works while recording too, so the owner can watch the file grow; the lock is released before Explorer
// is asked, for the same reason as Browse.
func (r *Recorder) OpenFolder(ctx context.Context) error {
	r.mu.Lock()
	folder := r.settings.Folder
	r.mu.Unlock()
	if err := r.deps.Files.CheckFolder(folder); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidCommand, err)
	}
	if err := r.deps.Opener.Open(ctx, folder); err != nil {
		return fmt.Errorf("open the folder: %w", err)
	}
	return nil
}

func (r *Recorder) Start(ctx context.Context, cmd StartCommand) (Status, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.deps.Clock.Now()
	// Checked on a copy first, so a refused Start creates no file.
	probe := r.session
	if err := probe.Start(now, true); err != nil {
		return r.statusLocked(), conflict(err)
	}
	if err := r.deps.Files.CheckFolder(r.settings.Folder); err != nil {
		r.folderProblem = err.Error()
		return r.statusLocked(), fmt.Errorf("%w: %w", ErrInvalidCommand, err)
	}
	r.folderProblem = ""
	r.refreshDevices(ctx)
	file, err := r.deps.Files.Create(r.settings.Folder, func(attempt int) string {
		return domain.FileName(now, cmd.Title, attempt)
	}, AudioOptions{NoiseCleanup: r.settings.NoiseCleanup})
	if err != nil {
		if errors.Is(err, ErrFolderNotWritable) {
			return r.statusLocked(), fmt.Errorf("%w: %w", ErrInvalidCommand, err)
		}
		return r.statusLocked(), fmt.Errorf("create recording: %w", err)
	}
	r.file = file
	r.notice = Notice{}
	opened := r.openMic(ctx, domain.ChooseMic(r.devices, "", domain.MicID(r.settings.MicID)))
	_ = r.session.Start(now, opened)
	r.deps.Awake.Hold()
	if !opened {
		r.notice = Notice{Kind: NoticeNoMic}
	}
	r.rememberMic(ctx)
	return r.statusLocked(), nil
}

// Pause keeps the mic open, so the level meter stays live and Resume is instant, and keeps the laptop awake.
func (r *Recorder) Pause(ctx context.Context) (Status, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.session.Pause(r.deps.Clock.Now()); err != nil {
		return r.statusLocked(), conflict(err)
	}
	r.flush(ctx)
	return r.statusLocked(), nil
}

func (r *Recorder) Resume(ctx context.Context) (Status, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.deps.Clock.Now()
	probe := r.session
	if err := probe.Resume(now, true); err != nil {
		return r.statusLocked(), conflict(err)
	}
	opened := r.stream != nil
	if !opened {
		r.refreshDevices(ctx)
		opened = r.openMic(ctx, domain.ChooseMic(r.devices, "", domain.MicID(r.settings.MicID)))
	}
	_ = r.session.Resume(now, opened)
	if !opened {
		r.notice = Notice{Kind: NoticeNoMic}
	}
	return r.statusLocked(), nil
}

// Stop returns no error when finalising fails: the recording has stopped either way, and the notice says why.
func (r *Recorder) Stop(ctx context.Context) (Status, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.stopLocked(ctx, true)
}

func (r *Recorder) Quit(ctx context.Context) error {
	r.mu.Lock()
	if r.session.State() != domain.Idle {
		_, _ = r.stopLocked(ctx, true)
	}
	r.mu.Unlock()
	r.deps.Quit()
	return nil
}

// stopLocked ends the recording. keepQueued writes the blocks still queued, unless a write has just failed.
func (r *Recorder) stopLocked(ctx context.Context, keepQueued bool) (Status, error) {
	wasRecording := r.session.State() == domain.Recording
	if err := r.session.Stop(r.deps.Clock.Now()); err != nil {
		return r.statusLocked(), conflict(err)
	}
	r.closeStream(ctx)
	r.drainFrames(ctx, keepQueued && wasRecording)
	r.finishFile(ctx)
	r.deps.Awake.Release()
	r.level = 0
	return r.statusLocked(), nil
}

// drainFrames writes the blocks still queued when recording, so the last words before Stop are kept; anything
// queued during a pause is dropped as a pause would have.
func (r *Recorder) drainFrames(ctx context.Context, write bool) {
	for {
		select {
		case block := <-r.frames:
			if !write {
				continue
			}
			if err := r.file.Write(block); err != nil {
				r.deps.Logger.ErrorContext(ctx, "write the last audio", "error", err)
				r.notice = Notice{Kind: noticeFor(err)}
				write = false
			}
		default:
			return
		}
	}
}

func (r *Recorder) finishFile(ctx context.Context) {
	name := r.file.Name()
	if err := r.file.Close(); err != nil {
		r.deps.Logger.ErrorContext(ctx, "finalise the recording", "file", name, "error", err)
		r.notice = Notice{Kind: noticeFor(err)}
	}
	r.lastFile = FileInfo{Name: name, Size: r.file.Size()}
	r.file = nil
}

func (r *Recorder) flush(ctx context.Context) {
	if err := r.file.Flush(); err != nil {
		r.deps.Logger.WarnContext(ctx, "flush the recording", "error", err)
	}
}

// openMic opens the chosen mic into the shared frames channel. A mic that will not open counts as no mic.
func (r *Recorder) openMic(ctx context.Context, choice domain.Choice) bool {
	mic, ok := choice.Mic()
	if !ok {
		r.micID = ""
		return false
	}
	stream, err := r.deps.Capture.Open(string(mic.ID()), r.frames)
	if err != nil {
		r.deps.Logger.WarnContext(ctx, "open the mic", "mic", mic.Name(), "error", err)
		r.micID = ""
		return false
	}
	r.stream = stream
	r.micID = mic.ID()
	return true
}

func (r *Recorder) closeStream(ctx context.Context) {
	if r.stream == nil {
		return
	}
	if err := r.stream.Close(); err != nil {
		r.deps.Logger.WarnContext(ctx, "close the mic", "error", err)
	}
	r.stream = nil
}

func (r *Recorder) rememberMic(ctx context.Context) {
	if r.micID == "" || string(r.micID) == r.settings.MicID {
		return
	}
	r.settings.MicID = string(r.micID)
	r.saveSettings(ctx)
}

// saveSettings never fails a use case: losing a remembered setting costs a click next launch, not a recording.
func (r *Recorder) saveSettings(ctx context.Context) {
	if err := r.deps.Settings.Save(r.settings); err != nil {
		r.deps.Logger.WarnContext(ctx, "save settings", "error", err)
	}
}

// refreshDevices keeps the last good list when listing fails, and says whether it got a new one.
func (r *Recorder) refreshDevices(ctx context.Context) bool {
	devices, err := r.deps.Mics.List(ctx)
	if err != nil {
		r.deps.Logger.WarnContext(ctx, "list mics", "error", err)
		return false
	}
	mics := make([]domain.Mic, 0, len(devices))
	for _, d := range devices {
		mics = append(mics, domain.NewMic(domain.MicID(d.ID), d.Name, d.IsDefault))
	}
	r.devices = mics
	return true
}

func (r *Recorder) folderCheck(folder string) string {
	if err := r.deps.Files.CheckFolder(folder); err != nil {
		return err.Error()
	}
	return ""
}

func (r *Recorder) statusLocked() Status {
	now := r.deps.Clock.Now()
	st := Status{
		State:         stateOf(r.session.State()),
		Recorded:      r.session.Recorded(now),
		Folder:        r.settings.Folder,
		FolderProblem: r.folderProblem,
		NoiseCleanup:  r.settings.NoiseCleanup,
		Mics:          devicesOf(r.devices),
		MicID:         r.settings.MicID,
		File:          r.lastFile,
		Notice:        r.notice,
		DefaultName:   domain.FileName(now, "", 1),
	}
	if r.file != nil {
		st.Level = r.level
		st.MicID = string(r.micID)
		st.File = FileInfo{Name: r.file.Name(), Size: r.file.Size()}
	}
	return st
}

func devicesOf(mics []domain.Mic) []Device {
	devices := make([]Device, 0, len(mics))
	for _, m := range mics {
		devices = append(devices, Device{ID: string(m.ID()), Name: m.Name(), IsDefault: m.IsDefault()})
	}
	return devices
}

// cleanFolder accepts a path pasted with Explorer's "Copy as path", which wraps it in quotes.
func cleanFolder(raw string) string {
	return strings.TrimSpace(strings.Trim(strings.TrimSpace(raw), `"`))
}

func conflict(err error) error {
	return fmt.Errorf("%w: %w", ErrConflict, err)
}

func noticeFor(err error) NoticeKind {
	if errors.Is(err, ErrDiskFull) {
		return NoticeDiskFull
	}
	return NoticeWriteFailed
}
