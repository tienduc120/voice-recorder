package domain

import "slices"

// MicID is the operating system's identifier for an input device, stable across renames.
type MicID string

type Mic struct {
	id        MicID
	name      string
	isDefault bool
}

func NewMic(id MicID, name string, isDefault bool) Mic {
	return Mic{id: id, name: name, isDefault: isDefault}
}

func (m Mic) ID() MicID { return m.id }

func (m Mic) Name() string { return m.name }

// IsDefault reports whether Windows currently routes its default input to this mic.
func (m Mic) IsDefault() bool { return m.isDefault }

// Reason says why ChooseMic picked what it picked, so the status line can tell the owner.
type Reason int

const (
	KeptCurrent Reason = iota
	UsedPreferred
	FellBackToDefault
	NoMic
)

type Choice struct {
	mic    Mic
	reason Reason
}

// Mic is false when no mic is connected.
func (c Choice) Mic() (Mic, bool) { return c.mic, c.reason != NoMic }

func (c Choice) Reason() Reason { return c.reason }

// ChooseMic keeps the mic in use for as long as it is connected, even when a newly plugged-in mic becomes Windows'
// default: switching is the owner's call. Without a current mic it takes the last-used one, then Windows' default,
// then any mic at all.
func ChooseMic(devices []Mic, current, preferred MicID) Choice {
	if m, ok := find(devices, current); ok {
		return Choice{mic: m, reason: KeptCurrent}
	}
	if m, ok := find(devices, preferred); ok {
		return Choice{mic: m, reason: UsedPreferred}
	}
	if i := slices.IndexFunc(devices, Mic.IsDefault); i >= 0 {
		return Choice{mic: devices[i], reason: FellBackToDefault}
	}
	if len(devices) > 0 {
		return Choice{mic: devices[0], reason: FellBackToDefault}
	}
	return Choice{reason: NoMic}
}

// NewlyConnected lists the mics in after whose ID was not in before, in after's order.
func NewlyConnected(before, after []Mic) []Mic {
	var added []Mic
	for _, m := range after {
		if _, ok := find(before, m.id); !ok {
			added = append(added, m)
		}
	}
	return added
}

func find(devices []Mic, id MicID) (Mic, bool) {
	if id == "" {
		return Mic{}, false
	}
	i := slices.IndexFunc(devices, func(m Mic) bool { return m.id == id })
	if i < 0 {
		return Mic{}, false
	}
	return devices[i], true
}
