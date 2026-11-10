// Package oggopus writes Opus packets into an Ogg file (RFC 3533, RFC 7845), syncing each page to disk.
package oggopus

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

const (
	// PacketsPerPage is one second of 20 ms packets: a page reaches the disk at least once a second of audio, which
	// bounds what a crash can lose.
	PacketsPerPage = 50

	capturePattern = "OggS"
	headerSize     = 27
	crcOffset      = 22
	maxSegments    = 255
	maxLacingValue = 255
	flagBOS        = 0x02
	flagEOS        = 0x04

	opusHeadVersion = 1
	monoChannels    = 1
	// inputRate is informational in OpusHead; Opus always decodes at 48 kHz.
	inputRate = 48000
	// maxPacketSamples is Opus's longest packet, 120 ms at 48 kHz.
	maxPacketSamples = 5760
	maxVendorBytes   = 1024
)

var errOutOfRange = errors.New("ogg opus: value out of range")

// Sink is where pages go; *os.File is one.
type Sink interface {
	Write(p []byte) (int, error)
	Sync() error
}

// Writer buffers packets into pages. A packet never continues onto another page: a cut-short file then loses only
// whole packets of its last page.
type Writer struct {
	out    Sink
	serial uint32
	seq    uint32
	// decoded is every 48 kHz sample the stream decodes to so far, pre-skip included; it is the granule position.
	decoded uint64
	lacing  []byte
	body    []byte
	packets int
}

// NewWriter writes the OpusHead and OpusTags header pages, each on its own page as RFC 7845 requires.
func NewWriter(out Sink, serial uint32, preSkip int, vendor string) (*Writer, error) {
	if preSkip < 0 || preSkip > math.MaxUint16 {
		return nil, fmt.Errorf("%w: pre-skip %d", errOutOfRange, preSkip)
	}
	vendorLen := len(vendor)
	if vendorLen > maxVendorBytes {
		return nil, fmt.Errorf("%w: vendor of %d bytes", errOutOfRange, vendorLen)
	}
	w := &Writer{out: out, serial: serial}
	head := opusHead(uint16(preSkip))
	if err := w.writePage(flagBOS, 0, lace(nil, len(head)), head); err != nil {
		return nil, err
	}
	tags := opusTags(vendor, uint32(vendorLen))
	if err := w.writePage(0, 0, lace(nil, len(tags)), tags); err != nil {
		return nil, err
	}
	return w, nil
}

// WritePacket adds a packet that decodes to samples 48 kHz samples. A full page goes to disk at once.
func (w *Writer) WritePacket(packet []byte, samples int) error {
	if samples <= 0 || samples > maxPacketSamples {
		return fmt.Errorf("%w: a packet of %d samples", errOutOfRange, samples)
	}
	if len(w.lacing)+len(packet)/maxLacingValue+1 > maxSegments {
		if err := w.Flush(); err != nil {
			return err
		}
	}
	w.lacing = lace(w.lacing, len(packet))
	w.body = append(w.body, packet...)
	w.decoded += uint64(samples)
	w.packets++
	if w.packets == PacketsPerPage {
		return w.Flush()
	}
	return nil
}

// Flush writes the buffered packets as a page and syncs it; with nothing buffered it does nothing.
func (w *Writer) Flush() error {
	if w.packets == 0 {
		return nil
	}
	return w.writeBuffered(0, w.decoded)
}

// Close writes the last page with the end-of-stream flag. finalGranule is pre-skip plus the real samples, which
// trims the zeros that padded the last packet (RFC 7845 §4.4).
func (w *Writer) Close(finalGranule int64) error {
	if finalGranule < 0 {
		return fmt.Errorf("%w: final granule %d", errOutOfRange, finalGranule)
	}
	return w.writeBuffered(flagEOS, uint64(finalGranule))
}

func (w *Writer) writeBuffered(flags byte, granule uint64) error {
	err := w.writePage(flags, granule, w.lacing, w.body)
	w.lacing, w.body, w.packets = w.lacing[:0], w.body[:0], 0
	return err
}

func (w *Writer) writePage(flags byte, granule uint64, lacing, body []byte) error {
	segments := len(lacing)
	if segments > maxSegments {
		return fmt.Errorf("%w: %d segments on one page", errOutOfRange, segments)
	}
	page := make([]byte, 0, headerSize+segments+len(body))
	page = append(page, capturePattern...)
	page = append(page, 0, flags)
	page = binary.LittleEndian.AppendUint64(page, granule)
	page = binary.LittleEndian.AppendUint32(page, w.serial)
	page = binary.LittleEndian.AppendUint32(page, w.seq)
	page = binary.LittleEndian.AppendUint32(page, 0)
	page = append(page, byte(segments))
	page = append(page, lacing...)
	page = append(page, body...)
	binary.LittleEndian.PutUint32(page[crcOffset:], CRC(page))

	if _, err := w.out.Write(page); err != nil {
		return fmt.Errorf("write ogg page: %w", err)
	}
	if err := w.out.Sync(); err != nil {
		return fmt.Errorf("sync ogg page: %w", err)
	}
	w.seq++
	return nil
}

// lace appends a packet's lacing values: 255 for each full segment, then the remainder, which may be 0.
func lace(lacing []byte, size int) []byte {
	for size >= maxLacingValue {
		lacing = append(lacing, maxLacingValue)
		size -= maxLacingValue
	}
	if size < 0 {
		// A length is never negative; the bound lets the conversion below be checked.
		size = 0
	}
	return append(lacing, byte(size))
}

func opusHead(preSkip uint16) []byte {
	head := []byte("OpusHead")
	head = append(head, opusHeadVersion, monoChannels)
	head = binary.LittleEndian.AppendUint16(head, preSkip)
	head = binary.LittleEndian.AppendUint32(head, inputRate)
	// Output gain 0 dB (2 bytes) and channel mapping family 0 (1 byte).
	return append(head, 0, 0, 0)
}

func opusTags(vendor string, vendorLen uint32) []byte {
	tags := binary.LittleEndian.AppendUint32([]byte("OpusTags"), vendorLen)
	tags = append(tags, vendor...)
	// No user comments.
	return binary.LittleEndian.AppendUint32(tags, 0)
}
