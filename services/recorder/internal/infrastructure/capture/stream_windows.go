package capture

import (
	"errors"
	"fmt"
	"sync"
	"time"
	"unsafe"

	"github.com/moutend/go-wca/pkg/wca"
	"golang.org/x/sys/windows"
)

// stream is one open mic. Only Run's thread calls its methods, except Lost and Close.
type stream struct {
	c        *Capture
	client   *wca.IAudioClient
	capture  *wca.IAudioCaptureClient
	event    windows.Handle
	blocks   *blocker
	buf      []float32
	watch    stallWatch
	lost     chan struct{}
	lostOnce sync.Once
}

func startStream(c *Capture, device *wca.IMMDevice, frames chan<- []float32) (*stream, error) {
	s := &stream{c: c, blocks: newBlocker(frames), watch: newStallWatch(time.Now()), lost: make(chan struct{})}
	if err := s.start(device); err != nil {
		s.release()
		return nil, err
	}
	return s, nil
}

func (s *stream) start(device *wca.IMMDevice) error {
	if err := device.Activate(wca.IID_IAudioClient, wca.CLSCTX_ALL, nil, &s.client); err != nil {
		return fmt.Errorf("capture: activate the mic: %w", err)
	}
	format, flags := streamFormat()
	if err := s.client.Initialize(wca.AUDCLNT_SHAREMODE_SHARED, flags, bufferDuration, 0, &format, nil); err != nil {
		return fmt.Errorf("capture: set the mic to 48 kHz mono: %w", err)
	}
	event, err := windows.CreateEvent(nil, 0, 0, nil)
	if err != nil {
		return fmt.Errorf("capture: buffer event: %w", err)
	}
	s.event = event
	if err = s.client.SetEventHandle(uintptr(event)); err != nil {
		return fmt.Errorf("capture: buffer event: %w", err)
	}
	if err = s.client.GetService(wca.IID_IAudioCaptureClient, &s.capture); err != nil {
		return fmt.Errorf("capture: capture client: %w", err)
	}
	if err = s.client.Start(); err != nil {
		return fmt.Errorf("capture: start the mic: %w", err)
	}
	return nil
}

// drain hands every packet waiting in the device buffer to the blocker and says how many there were.
func (s *stream) drain() (int, error) {
	for packets := 0; ; packets++ {
		var waiting uint32
		if err := s.capture.GetNextPacketSize(&waiting); err != nil {
			return packets, err
		}
		if waiting == 0 {
			return packets, nil
		}
		var data *byte
		var frames, flags uint32
		if err := s.capture.GetBuffer(&data, &frames, &flags, nil, nil); err != nil {
			return packets, err
		}
		s.buf = append(s.buf[:0], make([]float32, frames)...)
		if flags&silentBufferFlag == 0 && frames > 0 {
			copy(s.buf, unsafe.Slice((*float32)(unsafe.Pointer(data)), frames))
		}
		if err := s.capture.ReleaseBuffer(frames); err != nil {
			return packets, err
		}
		s.blocks.push(s.buf)
	}
}

func (s *stream) release() {
	if s.client != nil {
		_ = s.client.Stop()
	}
	if s.capture != nil {
		s.capture.Release()
	}
	if s.client != nil {
		s.client.Release()
	}
	if s.event != 0 {
		_ = windows.CloseHandle(s.event)
	}
}

func (s *stream) markLost() { s.lostOnce.Do(func() { close(s.lost) }) }

func (s *stream) Lost() <-chan struct{} { return s.lost }

func (s *stream) Close() error {
	err := s.c.do(func() {
		if s.c.current == s {
			s.c.closeCurrent()
		}
	})
	if errors.Is(err, errNotRunning) {
		// Run closed every stream when it stopped.
		return nil
	}
	return err
}
