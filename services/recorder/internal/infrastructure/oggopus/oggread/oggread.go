// Package oggread parses Ogg pages for tests; production code never imports it.
package oggread

import (
	"encoding/binary"

	"github.com/tienduc120/voice-recorder/services/recorder/internal/infrastructure/oggopus"
)

const (
	headerSize     = 27
	crcOffset      = 22
	crcSize        = 4
	segmentsOffset = 26
	maxLacingValue = 255
)

type Page struct {
	Flags    byte
	Granule  uint64
	Serial   uint32
	Sequence uint32
	Lacing   []byte
	Packets  [][]byte
}

// Pages returns every page in data up to the first that is cut short or fails its checksum, as a player would.
func Pages(data []byte) []Page {
	var pages []Page
	for len(data) >= headerSize && string(data[:4]) == "OggS" {
		segments := int(data[segmentsOffset])
		if len(data) < headerSize+segments {
			break
		}
		lacing := data[headerSize : headerSize+segments]
		bodySize := 0
		for _, l := range lacing {
			bodySize += int(l)
		}
		size := headerSize + segments + bodySize
		if len(data) < size || !checksumOK(data[:size]) {
			break
		}
		pages = append(pages, Page{
			Flags:    data[5],
			Granule:  binary.LittleEndian.Uint64(data[6:]),
			Serial:   binary.LittleEndian.Uint32(data[14:]),
			Sequence: binary.LittleEndian.Uint32(data[18:]),
			Lacing:   append([]byte(nil), lacing...),
			Packets:  packets(lacing, data[headerSize+segments:size]),
		})
		data = data[size:]
	}
	return pages
}

func checksumOK(page []byte) bool {
	want := binary.LittleEndian.Uint32(page[crcOffset:])
	scratch := append([]byte(nil), page...)
	clear(scratch[crcOffset : crcOffset+crcSize])
	return oggopus.CRC(scratch) == want
}

func packets(lacing, body []byte) [][]byte {
	var out [][]byte
	start, size := 0, 0
	for _, l := range lacing {
		size += int(l)
		if l < maxLacingValue {
			out = append(out, append([]byte(nil), body[start:start+size]...))
			start += size
			size = 0
		}
	}
	return out
}
