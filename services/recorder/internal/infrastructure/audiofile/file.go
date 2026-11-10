package audiofile

import (
	"errors"
	"fmt"

	"github.com/tienduc120/voice-recorder/services/recorder/internal/infrastructure/denoise"
	"github.com/tienduc120/voice-recorder/services/recorder/internal/infrastructure/dsp"
	"github.com/tienduc120/voice-recorder/services/recorder/internal/infrastructure/oggopus"
	"github.com/tienduc120/voice-recorder/services/recorder/internal/infrastructure/opusenc"
)

// maxPacketBytes is the largest Opus packet for one frame (RFC 6716).
const maxPacketBytes = 1275

// sink is where a recording's bytes go; *os.File is one.
type sink interface {
	oggopus.Sink
	Close() error
}

// countingSink knows the file's size without asking the disk.
type countingSink struct {
	sink

	written int64
}

func (c *countingSink) Write(p []byte) (int, error) {
	n, err := c.sink.Write(p)
	c.written += int64(n)
	return n, err
}

// file streams samples through high-pass, cleanup, leveller and encoder into Ogg pages. Nothing but a few frames is
// held in memory. After the first failure every call returns it: a recording with a hole is worse than one that
// stopped.
type file struct {
	out     *countingSink
	name    string
	ogg     *oggopus.Writer
	enc     Encoder
	den     Denoiser
	hp      *dsp.HighPass
	lev     *dsp.Leveller
	frames  *dsp.Framer
	toEnc   *dsp.Framer
	packet  []byte
	samples int64
	err     error
}

func newFile(out sink, name string, enc Encoder, den Denoiser) (*file, error) {
	counted := &countingSink{sink: out}
	ogg, err := oggopus.NewWriter(counted, serial(), enc.PreSkip(), enc.Vendor())
	if err != nil {
		return nil, writeError(err)
	}
	return &file{
		out:    counted,
		name:   name,
		ogg:    ogg,
		enc:    enc,
		den:    den,
		hp:     dsp.NewHighPass(),
		lev:    dsp.NewLeveller(),
		frames: dsp.NewFramer(denoise.FrameSize),
		toEnc:  dsp.NewFramer(opusenc.FrameSize),
		packet: make([]byte, maxPacketBytes),
	}, nil
}

func (f *file) Name() string { return f.name }

func (f *file) Size() int64 { return f.out.written }

func (f *file) Write(samples []float32) error {
	if f.err != nil {
		return f.err
	}
	f.samples += int64(len(samples))
	return f.push(samples)
}

func (f *file) Flush() error {
	if f.err != nil {
		return f.err
	}
	if err := f.ogg.Flush(); err != nil {
		f.err = writeError(err)
	}
	return f.err
}

// Close pushes enough silence through for the encoder's lookahead to release the last real audio, ends the Ogg
// stream at exactly the samples written (RFC 7845 end trimming), and always closes the file.
func (f *file) Close() error {
	var err error
	if f.err == nil {
		err = f.finish()
	}
	return errors.Join(err, f.enc.Close(), f.out.Close())
}

func (f *file) finish() error {
	preSkip := int64(f.enc.PreSkip())
	covered := f.samples + preSkip
	padding := preSkip + (opusenc.FrameSize-covered%opusenc.FrameSize)%opusenc.FrameSize
	if err := f.push(make([]float32, padding)); err != nil {
		return err
	}
	if err := f.ogg.Close(covered); err != nil {
		return writeError(err)
	}
	return nil
}

func (f *file) push(samples []float32) error {
	if err := f.frames.Push(samples, f.stage); err != nil {
		f.err = err
	}
	return f.err
}

// stage runs one 10 ms frame through the signal chain, in the spec's order.
func (f *file) stage(frame []float32) error {
	f.hp.Process(frame)
	if f.den != nil {
		if err := f.den.Process(frame); err != nil {
			return fmt.Errorf("noise cleanup: %w", err)
		}
	}
	f.lev.Process(frame)
	return f.toEnc.Push(frame, f.encode)
}

func (f *file) encode(frame []float32) error {
	n, err := f.enc.Encode(frame, f.packet)
	if err != nil {
		return fmt.Errorf("encode: %w", err)
	}
	if err = f.ogg.WritePacket(f.packet[:n], opusenc.FrameSize); err != nil {
		return writeError(err)
	}
	return nil
}
