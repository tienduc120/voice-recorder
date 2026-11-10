package application_test

import (
	"context"
	"log/slog"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/tienduc120/voice-recorder/services/recorder/internal/application"
)

const (
	builtInName = "Microphone Array (Realtek)"
	usbName     = "USB Microphone"
	recFolder   = `C:\Rec`
	fakeSize    = 1234
	blockLen    = 480
)

// events records what the fakes did, in order, across all of them.
type events struct {
	mu   sync.Mutex
	list []string
}

func (e *events) add(name string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.list = append(e.list, name)
}

func (e *events) all() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return slices.Clone(e.list)
}

type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

type fakeMics struct {
	mu      sync.Mutex
	devices []application.Device
	err     error
	calls   int
}

func (m *fakeMics) List(context.Context) ([]application.Device, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	return slices.Clone(m.devices), m.err
}

func (m *fakeMics) set(devices ...application.Device) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.devices = devices
}

type fakeStream struct {
	id     string
	ev     *events
	lost   chan struct{}
	mu     sync.Mutex
	closes int
}

func (s *fakeStream) Lost() <-chan struct{} { return s.lost }

func (s *fakeStream) Close() error {
	s.mu.Lock()
	s.closes++
	s.mu.Unlock()
	s.ev.add("stream.close")
	return nil
}

func (s *fakeStream) closeCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closes
}

type fakeCapture struct {
	ev      *events
	mu      sync.Mutex
	opened  []string
	streams []*fakeStream
	frames  chan<- []float32
	err     error
}

func (c *fakeCapture) Open(deviceID string, frames chan<- []float32) (application.Stream, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.opened = append(c.opened, deviceID)
	if c.err != nil {
		return nil, c.err
	}
	c.frames = frames
	s := &fakeStream{id: deviceID, ev: c.ev, lost: make(chan struct{})}
	c.streams = append(c.streams, s)
	c.ev.add("capture.open " + deviceID)
	return s, nil
}

func (c *fakeCapture) openedIDs() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.opened)
}

func (c *fakeCapture) stream(i int) *fakeStream {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.streams[i]
}

// send hands the recorder one block, as the capture adapter would.
func (c *fakeCapture) send(value float32) {
	c.mu.Lock()
	frames := c.frames
	c.mu.Unlock()
	block := make([]float32, blockLen)
	for i := range block {
		block[i] = value
	}
	frames <- block
}

type fakeFile struct {
	name     string
	ev       *events
	mu       sync.Mutex
	writes   int
	flushes  int
	closes   int
	writeErr error
	closeErr error
}

func (f *fakeFile) Name() string { return f.name }

func (f *fakeFile) Size() int64 { return fakeSize }

func (f *fakeFile) Write([]float32) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.writeErr != nil {
		return f.writeErr
	}
	f.writes++
	return nil
}

func (f *fakeFile) Flush() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.flushes++
	return nil
}

func (f *fakeFile) Close() error {
	f.mu.Lock()
	f.closes++
	err := f.closeErr
	f.mu.Unlock()
	f.ev.add("file.close")
	return err
}

type fileCounts struct{ writes, flushes, closes int }

func (f *fakeFile) counts() fileCounts {
	f.mu.Lock()
	defer f.mu.Unlock()
	return fileCounts{writes: f.writes, flushes: f.flushes, closes: f.closes}
}

type fakeFiles struct {
	ev       *events
	mu       sync.Mutex
	checked  []string
	created  []string
	names    []func(int) string
	opts     []application.AudioOptions
	files    []*fakeFile
	checkErr error
}

func (f *fakeFiles) CheckFolder(folder string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.checked = append(f.checked, folder)
	return f.checkErr
}

func (f *fakeFiles) Create(
	folder string,
	name func(attempt int) string,
	opts application.AudioOptions,
) (application.AudioFile, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.created = append(f.created, folder)
	f.names = append(f.names, name)
	f.opts = append(f.opts, opts)
	file := &fakeFile{name: name(1), ev: f.ev}
	f.files = append(f.files, file)
	return file, nil
}

func (f *fakeFiles) createdFolders() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.created)
}

func (f *fakeFiles) first() *fakeFile {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.files[0]
}

type fakeSettings struct {
	mu      sync.Mutex
	stored  application.Settings
	saved   []application.Settings
	loadErr error
}

func (s *fakeSettings) Load() (application.Settings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stored, s.loadErr
}

func (s *fakeSettings) Save(settings application.Settings) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.saved = append(s.saved, settings)
	s.stored = settings
	return nil
}

func (s *fakeSettings) saves() []application.Settings {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.saved)
}

type fakePicker struct {
	folder string
	chosen bool
	start  string
}

func (p *fakePicker) Pick(_ context.Context, start string) (string, bool, error) {
	p.start = start
	return p.folder, p.chosen, nil
}

type fakeAwake struct {
	mu       sync.Mutex
	holds    int
	releases int
}

func (a *fakeAwake) Hold() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.holds++
}

func (a *fakeAwake) Release() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.releases++
}

type awakeCounts struct{ holds, releases int }

func (a *fakeAwake) counts() awakeCounts {
	a.mu.Lock()
	defer a.mu.Unlock()
	return awakeCounts{holds: a.holds, releases: a.releases}
}

type deps struct {
	clock    application.Clock
	mics     *fakeMics
	capture  *fakeCapture
	files    *fakeFiles
	settings *fakeSettings
	picker   *fakePicker
	awake    *fakeAwake
	ev       *events
}

func builtIn(isDefault bool) application.Device {
	return application.Device{ID: "builtin", Name: builtInName, IsDefault: isDefault}
}

func usb(isDefault bool) application.Device {
	return application.Device{ID: "usb", Name: usbName, IsDefault: isDefault}
}

// newDeps starts from a remembered folder and mic, with the built-in mic as Windows' default and a USB mic beside it.
func newDeps(clock application.Clock) *deps {
	ev := &events{}
	return &deps{
		clock:    clock,
		mics:     &fakeMics{devices: []application.Device{builtIn(true), usb(false)}},
		capture:  &fakeCapture{ev: ev},
		files:    &fakeFiles{ev: ev},
		settings: &fakeSettings{stored: application.Settings{Folder: recFolder, MicID: "usb", NoiseCleanup: true}},
		picker:   &fakePicker{},
		awake:    &fakeAwake{},
		ev:       ev,
	}
}

func (d *deps) Deps() application.Deps {
	return application.Deps{
		Clock:    d.clock,
		Mics:     d.mics,
		Capture:  d.capture,
		Files:    d.files,
		Settings: d.settings,
		Picker:   d.picker,
		Awake:    d.awake,
		Logger:   slog.New(slog.DiscardHandler),
		Quit:     func() { d.ev.add("quit") },
	}
}

func meetingClock() *fakeClock {
	return &fakeClock{now: time.Date(2026, time.November, 9, 10, 30, 0, 0, time.Local)}
}

func mustStatus(t *testing.T) func(application.Status, error) application.Status {
	t.Helper()
	return func(st application.Status, err error) application.Status {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		return st
	}
}
