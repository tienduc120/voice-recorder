package capture

import "time"

// stallLimit is how long a running stream may deliver nothing before it counts as lost. A capture endpoint sends a
// packet, a silent one included, every 10 ms or so; a driver that hangs after sleep sends none and reports no error.
const stallLimit = 3 * time.Second

// stallWatch notices a stream that stopped delivering without saying so.
type stallWatch struct {
	heard time.Time
}

func newStallWatch(opened time.Time) stallWatch {
	return stallWatch{heard: opened}
}

// observe takes how many packets a drain brought and reports whether the stream has been silent too long.
func (w *stallWatch) observe(packets int, now time.Time) bool {
	if packets > 0 {
		w.heard = now
		return false
	}
	return now.Sub(w.heard) >= stallLimit
}
