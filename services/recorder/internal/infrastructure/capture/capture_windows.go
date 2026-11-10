package capture

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"time"

	"github.com/go-ole/go-ole"
	"github.com/moutend/go-wca/pkg/wca"
	"golang.org/x/sys/windows"

	"github.com/tienduc120/voice-recorder/services/recorder/internal/application"
	"github.com/tienduc120/voice-recorder/services/recorder/internal/infrastructure/dsp"
)

const (
	// openTimeout bounds how long Open waits for the capture loop to start.
	openTimeout = 5 * time.Second
	// bufferDuration is WASAPI's buffer in 100-ns units: 200 ms of slack before the device overruns.
	bufferDuration = 200 * 10000
	// deviceInvalidated is AUDCLNT_E_DEVICE_INVALIDATED: the endpoint was unplugged or reset.
	deviceInvalidated = 0x88890004
	// sFalse is S_FALSE, which CoInitializeEx returns when COM is already initialised on the thread.
	sFalse = 1
	// rpcChangedMode is RPC_E_CHANGED_MODE: COM is initialised on the thread in the other model, and usable.
	rpcChangedMode   = 0x80010106
	waveFormatFloat  = 3
	monoChannels     = 1
	bytesPerSample   = 4
	bitsPerSample    = 32
	silentBufferFlag = 0x2
	// jobQueue holds requests until the loop wakes; the recorder sends at most one at a time.
	jobQueue = 8
)

var errNotRunning = errors.New("capture: the capture loop is not running")

// Capture owns the mic streams. COM objects belong to the thread that made them, so every stream lives on Run's
// locked thread; Open and Close hand their work to it.
type Capture struct {
	logger  *slog.Logger
	jobs    chan func()
	started chan struct{}
	stopped chan struct{}

	// Only Run's thread touches these.
	wake    windows.Handle
	enum    *wca.IMMDeviceEnumerator
	current *stream
}

func New(logger *slog.Logger) *Capture {
	return &Capture{
		logger:  logger,
		jobs:    make(chan func(), jobQueue),
		started: make(chan struct{}),
		stopped: make(chan struct{}),
	}
}

// Run serves streams until ctx ends, then closes the open one and returns nil.
func (c *Capture) Run(ctx context.Context) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	uninit, err := initCOM(ole.COINIT_APARTMENTTHREADED)
	if err != nil {
		return err
	}
	defer uninit()
	wake, err := windows.CreateEvent(nil, 0, 0, nil)
	if err != nil {
		return fmt.Errorf("capture: wake event: %w", err)
	}
	defer func() { _ = windows.CloseHandle(wake) }()
	if err = wca.CoCreateInstance(
		wca.CLSID_MMDeviceEnumerator, 0, wca.CLSCTX_ALL, wca.IID_IMMDeviceEnumerator, &c.enum,
	); err != nil {
		return fmt.Errorf("capture: device enumerator: %w", err)
	}
	defer c.enum.Release()
	c.wake = wake
	stopWaking := context.AfterFunc(ctx, func() { _ = windows.SetEvent(wake) })
	defer stopWaking()

	close(c.started)
	defer close(c.stopped)
	defer c.closeCurrent()
	return c.loop(ctx)
}

func (c *Capture) loop(ctx context.Context) error {
	for {
		handles := []windows.Handle{c.wake}
		if c.current != nil {
			handles = append(handles, c.current.event)
		}
		if _, err := windows.WaitForMultipleObjects(handles, false, windows.INFINITE); err != nil {
			return fmt.Errorf("capture: wait: %w", err)
		}
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		c.runJobs()
		c.drainCurrent()
	}
}

func (c *Capture) runJobs() {
	for {
		select {
		case job := <-c.jobs:
			job()
		default:
			return
		}
	}
}

func (c *Capture) drainCurrent() {
	if c.current == nil {
		return
	}
	if err := c.current.drain(); err != nil {
		lost := c.current
		c.logger.Warn("the mic stopped", "error", err, "invalidated", isInvalidated(err))
		c.closeCurrent()
		lost.markLost()
	}
}

func (c *Capture) closeCurrent() {
	if c.current == nil {
		return
	}
	if drops := c.current.blocks.dropped(); drops > 0 {
		c.logger.Warn("the recorder fell behind the mic", "dropped_blocks", drops)
	}
	c.current.release()
	c.current = nil
}

// do runs job on Run's thread and waits for it.
func (c *Capture) do(job func()) error {
	select {
	case <-c.started:
	case <-time.After(openTimeout):
		return errNotRunning
	}
	done := make(chan struct{})
	select {
	case c.jobs <- func() { job(); close(done) }:
	case <-c.stopped:
		return errNotRunning
	}
	_ = windows.SetEvent(c.wake)
	select {
	case <-done:
		return nil
	case <-c.stopped:
		return errNotRunning
	}
}

// Open starts deviceID, closing any stream that is open: there is one mic at a time.
func (c *Capture) Open(deviceID string, frames chan<- []float32) (application.Stream, error) {
	var s *stream
	var openErr error
	if err := c.do(func() { s, openErr = c.open(deviceID, frames) }); err != nil {
		return nil, err
	}
	if openErr != nil {
		return nil, openErr
	}
	return s, nil
}

func (c *Capture) open(deviceID string, frames chan<- []float32) (*stream, error) {
	c.closeCurrent()
	device, err := findDevice(c.enum, deviceID)
	if err != nil {
		return nil, err
	}
	defer device.Release()
	s, err := startStream(c, device, frames)
	if err != nil {
		return nil, err
	}
	c.current = s
	return s, nil
}

// List enumerates the active mics on the caller's thread, which it locks for COM while it works.
func (c *Capture) List(context.Context) ([]application.Device, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	uninit, err := initCOM(ole.COINIT_MULTITHREADED)
	if err != nil {
		return nil, err
	}
	defer uninit()
	var enum *wca.IMMDeviceEnumerator
	if err = wca.CoCreateInstance(
		wca.CLSID_MMDeviceEnumerator, 0, wca.CLSCTX_ALL, wca.IID_IMMDeviceEnumerator, &enum,
	); err != nil {
		return nil, fmt.Errorf("capture: device enumerator: %w", err)
	}
	defer enum.Release()
	return c.listDevices(enum)
}

func (c *Capture) listDevices(enum *wca.IMMDeviceEnumerator) ([]application.Device, error) {
	defaultID := ""
	var def *wca.IMMDevice
	if enum.GetDefaultAudioEndpoint(wca.ECapture, wca.EConsole, &def) == nil {
		_ = def.GetId(&defaultID)
		def.Release()
	}
	var devices []application.Device
	err := eachActive(enum, func(d *wca.IMMDevice, id string) bool {
		devices = append(devices, application.Device{ID: id, Name: friendlyName(d), IsDefault: id == defaultID})
		return true
	})
	return devices, err
}

// friendlyName needs no PropVariantClear: go-wca's PROPVARIANT.String frees the string it reads.
func friendlyName(d *wca.IMMDevice) string {
	var props *wca.IPropertyStore
	if err := d.OpenPropertyStore(wca.STGM_READ, &props); err != nil {
		return "Microphone"
	}
	defer props.Release()
	var pv wca.PROPVARIANT
	if err := props.GetValue(&wca.PKEY_Device_FriendlyName, &pv); err != nil {
		return "Microphone"
	}
	return pv.String()
}

// findDevice exists because go-wca's IMMDeviceEnumerator.GetDevice takes no ID.
func findDevice(enum *wca.IMMDeviceEnumerator, deviceID string) (*wca.IMMDevice, error) {
	var found *wca.IMMDevice
	err := eachActive(enum, func(d *wca.IMMDevice, id string) bool {
		if id != deviceID {
			return true
		}
		d.AddRef()
		found = d
		return false
	})
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, fmt.Errorf("capture: no connected mic has the id %q", deviceID)
	}
	return found, nil
}

// eachActive calls visit for every active capture endpoint until it returns false; each device is released after.
func eachActive(enum *wca.IMMDeviceEnumerator, visit func(d *wca.IMMDevice, id string) bool) error {
	var collection *wca.IMMDeviceCollection
	if err := enum.EnumAudioEndpoints(wca.ECapture, wca.DEVICE_STATE_ACTIVE, &collection); err != nil {
		return fmt.Errorf("capture: list mics: %w", err)
	}
	defer collection.Release()
	var count uint32
	if err := collection.GetCount(&count); err != nil {
		return fmt.Errorf("capture: count mics: %w", err)
	}
	for i := range count {
		var d *wca.IMMDevice
		if err := collection.Item(i, &d); err != nil {
			return fmt.Errorf("capture: mic %d: %w", i, err)
		}
		var id string
		err := d.GetId(&id)
		keepGoing := err == nil && visit(d, id)
		d.Release()
		if !keepGoing && err == nil {
			return nil
		}
	}
	return nil
}

func streamFormat() (wca.WAVEFORMATEX, uint32) {
	return wca.WAVEFORMATEX{
		WFormatTag:      waveFormatFloat,
		NChannels:       monoChannels,
		NSamplesPerSec:  dsp.SampleRate,
		NAvgBytesPerSec: dsp.SampleRate * bytesPerSample,
		NBlockAlign:     bytesPerSample,
		WBitsPerSample:  bitsPerSample,
	}, wca.AUDCLNT_STREAMFLAGS_EVENTCALLBACK |
		wca.AUDCLNT_STREAMFLAGS_AUTOCONVERTPCM |
		wca.AUDCLNT_STREAMFLAGS_SRC_DEFAULT_QUALITY
}

// initCOM initialises COM on the locked thread and returns how to undo it.
func initCOM(model uint32) (func(), error) {
	err := ole.CoInitializeEx(0, model)
	var oleErr *ole.OleError
	switch {
	case err == nil, errors.As(err, &oleErr) && oleErr.Code() == sFalse:
		return ole.CoUninitialize, nil
	case errors.As(err, &oleErr) && oleErr.Code() == rpcChangedMode:
		return func() {}, nil
	default:
		return nil, fmt.Errorf("capture: initialise COM: %w", err)
	}
}

func isInvalidated(err error) bool {
	var oleErr *ole.OleError
	return errors.As(err, &oleErr) && oleErr.Code() == deviceInvalidated
}
