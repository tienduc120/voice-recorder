package application

import (
	"context"
	"math"
	"time"

	"github.com/tienduc120/voice-recorder/services/recorder/internal/domain"
)

const (
	// devicePollInterval is how fast a plugged-in or unplugged mic shows up; the stream's own Lost signal is faster.
	devicePollInterval = 2 * time.Second
	// meterFloorDB is the quietest level the meter shows; the scale runs from it to 0 dBFS.
	meterFloorDB = -60.0
	decibels     = 20
	// meterFallPerSecond lets the meter rise at once and fall at 24 dB/s, like a peak meter: a reading taken
	// between two words still shows the last one.
	meterFallPerSecond = 0.4
	// sampleRate is the rate the Capture port promises.
	sampleRate = 48000
)

// Run consumes frames and watches the mics until ctx ends, then finalises an open recording and returns nil.
func (r *Recorder) Run(ctx context.Context) error {
	ticker := time.NewTicker(devicePollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			r.shutdown(ctx)
			return nil
		case block := <-r.frames:
			r.consume(ctx, block)
		case <-ticker.C:
			r.checkDevices(ctx)
		case <-r.lost():
			r.checkDevices(ctx)
		}
	}
}

func (r *Recorder) shutdown(ctx context.Context) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.session.State() != domain.Idle {
		_, _ = r.stopLocked(ctx, true)
	}
}

func (r *Recorder) consume(ctx context.Context, block []float32) {
	r.mu.Lock()
	defer r.mu.Unlock()
	fall := meterFallPerSecond * float64(len(block)) / sampleRate
	r.level = math.Max(meterLevel(block), math.Max(0, r.level-fall))
	if r.session.State() != domain.Recording {
		return
	}
	if err := r.file.Write(block); err != nil {
		// A failed write ends the recording: the file is finalised as far as it goes and the owner is told.
		r.deps.Logger.ErrorContext(ctx, "write the recording", "error", err)
		_, _ = r.stopLocked(ctx, false)
		r.notice = Notice{Kind: noticeFor(err)}
	}
}

// lost is the open stream's Lost channel, or nil, which never fires, when no stream is open.
func (r *Recorder) lost() <-chan struct{} {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.stream == nil {
		return nil
	}
	return r.stream.Lost()
}

func meterLevel(block []float32) float64 {
	var peak float64
	for _, s := range block {
		peak = math.Max(peak, math.Abs(float64(s)))
	}
	if peak == 0 {
		return 0
	}
	level := (decibels*math.Log10(peak) - meterFloorDB) / -meterFloorDB
	return math.Max(0, math.Min(1, level))
}

// Level is the input level for the meter, from 0 to 1, and 0 while idle.
func (r *Recorder) Level(context.Context) float64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.file == nil {
		return 0
	}
	return r.level
}
