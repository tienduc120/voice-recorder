// Package opustest decodes Opus packets for tests with go-opus; production code never imports it.
package opustest

import (
	"fmt"

	gopus "github.com/skrashevich/go-opus"
)

// maxPacketSamples is Opus's longest packet, 120 ms at 48 kHz.
const maxPacketSamples = 5760

// Decode turns mono 48 kHz packets back into samples. It uses go-opus because opus-go's own decoder trips a libopus
// assertion on valid packets (ADR-0019).
func Decode(packets [][]byte) ([]int16, error) {
	d := gopus.NewDecoder()
	buf := make([]int16, maxPacketSamples)
	var out []int16
	for i, p := range packets {
		n, err := d.DecodeToInt16(p, buf)
		if err != nil {
			return nil, fmt.Errorf("packet %d: %w", i, err)
		}
		out = append(out, buf[:n]...)
	}
	return out, nil
}
