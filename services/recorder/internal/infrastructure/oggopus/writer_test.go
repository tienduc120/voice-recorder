package oggopus_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"reflect"
	"testing"

	"github.com/tienduc120/voice-recorder/services/recorder/internal/infrastructure/oggopus"
	"github.com/tienduc120/voice-recorder/services/recorder/internal/infrastructure/oggopus/oggread"
)

const (
	preSkip    = 312
	vendor     = "voice-recorder test"
	serial     = 7
	frame      = 960
	flagBOS    = 0x02
	flagEOS    = 0x04
	packetSize = 60
)

type fakeSink struct {
	bytes.Buffer

	syncs int
	err   error
}

func (s *fakeSink) Write(p []byte) (int, error) {
	if s.err != nil {
		return 0, s.err
	}
	return s.Buffer.Write(p)
}

func (s *fakeSink) Sync() error {
	s.syncs++
	return nil
}

func newWriter(t *testing.T, sink *fakeSink) *oggopus.Writer {
	t.Helper()
	w, err := oggopus.NewWriter(sink, serial, preSkip, vendor)
	if err != nil {
		t.Fatal(err)
	}
	return w
}

func packet(i int) []byte {
	p := make([]byte, packetSize)
	for j := range p {
		p[j] = byte(i + j)
	}
	return p
}

func writePackets(t *testing.T, w *oggopus.Writer, from, to int) {
	t.Helper()
	for i := from; i < to; i++ {
		if err := w.WritePacket(packet(i), frame); err != nil {
			t.Fatal(err)
		}
	}
}

func audioPackets(pages []oggread.Page) [][]byte {
	var out [][]byte
	for _, p := range pages[2:] {
		out = append(out, p.Packets...)
	}
	return out
}

func TestCRCMatchesTheOggCheckValue(t *testing.T) {
	t.Parallel()

	if got := oggopus.CRC([]byte("123456789")); got != 0x89a1897f {
		t.Errorf("CRC = %#x, want 0x89a1897f: Ogg's CRC is unreflected with no final XOR, unlike hash/crc32", got)
	}
}

func TestTheFileStartsWithOpusHeadAndOpusTags(t *testing.T) {
	t.Parallel()

	sink := &fakeSink{}
	newWriter(t, sink)
	pages := oggread.Pages(sink.Bytes())

	head := []byte("OpusHead")
	head = append(head, 1, 1)
	head = binary.LittleEndian.AppendUint16(head, preSkip)
	head = binary.LittleEndian.AppendUint32(head, 48000)
	head = append(head, 0, 0, 0)
	tags := binary.LittleEndian.AppendUint32([]byte("OpusTags"), uint32(len(vendor)))
	tags = binary.LittleEndian.AppendUint32(append(tags, vendor...), 0)

	if len(pages) != 2 {
		t.Fatalf("got %d pages, want the two header pages", len(pages))
	}
	if p := pages[0]; p.Flags != flagBOS || p.Granule != 0 || p.Sequence != 0 || p.Serial != serial ||
		!reflect.DeepEqual(p.Packets, [][]byte{head}) {
		t.Errorf("first page %+v, want a beginning-of-stream page holding only OpusHead %v", p, head)
	}
	if p := pages[1]; p.Flags != 0 || p.Granule != 0 || p.Sequence != 1 ||
		!reflect.DeepEqual(p.Packets, [][]byte{tags}) {
		t.Errorf("second page %+v, want a page holding only OpusTags", p)
	}
}

func TestAPageGoesToDiskEverySecondOfAudio(t *testing.T) {
	t.Parallel()

	sink := &fakeSink{}
	w := newWriter(t, sink)

	writePackets(t, w, 0, 50)
	pages := oggread.Pages(sink.Bytes())

	if len(pages) != 3 || pages[2].Granule != 50*frame || len(pages[2].Packets) != 50 || sink.syncs != 3 {
		t.Fatalf("pages %d, syncs %d: want one synced audio page after 1 s (granule 48000)", len(pages), sink.syncs)
	}

	writePackets(t, w, 50, 51)

	if got := len(oggread.Pages(sink.Bytes())); got != 3 {
		t.Errorf("pages = %d after one more packet, want 3: a page holds up to a second", got)
	}
}

func TestFlushWritesWhatIsBuffered(t *testing.T) {
	t.Parallel()

	sink := &fakeSink{}
	w := newWriter(t, sink)
	writePackets(t, w, 0, 15)

	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}
	pages := oggread.Pages(sink.Bytes())
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}

	if len(pages) != 3 || pages[2].Granule != 15*frame {
		t.Fatalf("got %d pages, want an audio page with granule 14400", len(pages))
	}
	if again := oggread.Pages(sink.Bytes()); len(again) != 3 {
		t.Errorf("a second Flush with nothing buffered wrote %d pages, want none", len(again)-3)
	}
}

func TestLongPacketsAreLacedAcrossSegments(t *testing.T) {
	t.Parallel()

	sink := &fakeSink{}
	w := newWriter(t, sink)
	long := bytes.Repeat([]byte{7}, 600)

	if err := w.WritePacket(long, frame); err != nil {
		t.Fatal(err)
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}

	p := oggread.Pages(sink.Bytes())[2]
	if !reflect.DeepEqual(p.Lacing, []byte{255, 255, 90}) || !reflect.DeepEqual(p.Packets, [][]byte{long}) {
		t.Errorf("lacing %v, want [255 255 90] and the packet read back whole", p.Lacing)
	}
}

func TestCloseMarksTheEndAndTrimsTheGranule(t *testing.T) {
	t.Parallel()

	sink := &fakeSink{}
	w := newWriter(t, sink)
	writePackets(t, w, 0, 3)

	if err := w.Close(preSkip + 2000); err != nil {
		t.Fatal(err)
	}

	pages := oggread.Pages(sink.Bytes())
	last := pages[len(pages)-1]
	if last.Flags&flagEOS == 0 || last.Granule != preSkip+2000 || len(audioPackets(pages)) != 3 {
		t.Errorf("last page %+v: want end of stream, granule %d, and all 3 packets", last, preSkip+2000)
	}
}

func TestCloseWithNothingBufferedStillEndsTheStream(t *testing.T) {
	t.Parallel()

	sink := &fakeSink{}
	w := newWriter(t, sink)
	writePackets(t, w, 0, 50)

	if err := w.Close(50 * frame); err != nil {
		t.Fatal(err)
	}

	pages := oggread.Pages(sink.Bytes())
	last := pages[len(pages)-1]
	if len(pages) != 4 || last.Flags&flagEOS == 0 || last.Granule != 50*frame || len(last.Packets) != 0 {
		t.Errorf("pages %d, last %+v: want an empty end-of-stream page after the full one", len(pages), last)
	}
}

func TestAFileCutShortKeepsEveryCompletePage(t *testing.T) {
	t.Parallel()

	sink := &fakeSink{}
	w := newWriter(t, sink)
	writePackets(t, w, 0, 120)
	full := sink.Len()
	if err := w.Close(120*frame - 100); err != nil {
		t.Fatal(err)
	}

	pages := oggread.Pages(sink.Bytes()[:full+10])

	var want [][]byte
	for i := range 100 {
		want = append(want, packet(i))
	}
	if len(pages) != 4 || !reflect.DeepEqual(audioPackets(pages), want) {
		t.Errorf("got %d pages, want the two header pages and both complete audio pages with their 100 packets",
			len(pages))
	}
}

func TestASinkFailureIsReturned(t *testing.T) {
	t.Parallel()

	sink := &fakeSink{}
	w := newWriter(t, sink)
	boom := errors.New("disk gone")
	sink.err = boom

	writePackets(t, w, 0, 49)
	err := w.WritePacket(packet(49), frame)

	if !errors.Is(err, boom) {
		t.Errorf("err = %v, want the sink's error", err)
	}
}

func TestValuesAnOggOpusFileCannotHoldAreRefused(t *testing.T) {
	t.Parallel()

	if _, err := oggopus.NewWriter(&fakeSink{}, serial, 70000, vendor); err == nil {
		t.Error("a pre-skip beyond 16 bits must be refused, not truncated")
	}
	w := newWriter(t, &fakeSink{})
	if err := w.WritePacket(packet(0), 0); err == nil {
		t.Error("a packet of no samples must be refused: it would corrupt every later granule")
	}
	if err := w.Close(-1); err == nil {
		t.Error("a negative final granule must be refused")
	}
}
