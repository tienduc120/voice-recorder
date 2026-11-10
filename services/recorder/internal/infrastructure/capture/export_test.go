//go:build windows

package capture

import "github.com/moutend/go-wca/pkg/wca"

// StreamFormat is the format and flags every stream asks WASAPI for.
func StreamFormat() (wca.WAVEFORMATEX, uint32) { return streamFormat() }
