package application

import (
	"context"
	"fmt"
	"slices"

	"github.com/tienduc120/voice-recorder/services/recorder/internal/domain"
)

type ChooseMicCommand struct {
	ID string
}

// ChooseMic works at any time. While recording it switches streams into the same file; the gap is the new
// device's start-up. A mic that will not start is neither used nor remembered.
func (r *Recorder) ChooseMic(ctx context.Context, cmd ChooseMicCommand) (Status, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id := domain.MicID(cmd.ID)
	if !r.listed(id) {
		return r.statusLocked(), fmt.Errorf("%w: no connected mic has the id %q", ErrInvalidCommand, cmd.ID)
	}
	if r.session.State() != domain.Idle {
		previous := r.micID
		r.closeStream(ctx)
		if !r.openMic(ctx, domain.ChooseMic(r.devices, id, id)) {
			// The old mic goes back in, so one click on a busy mic never silences a meeting.
			r.reopen(ctx, domain.ChooseMic(r.devices, previous, domain.MicID(r.settings.MicID)), "")
			return r.statusLocked(), fmt.Errorf("%w: %q would not start; another app may be using it",
				ErrConflict, nameOf(r.devices, id))
		}
		r.session.MicBack(r.deps.Clock.Now())
	}
	r.settings.MicID = cmd.ID
	r.saveSettings(ctx)
	return r.statusLocked(), nil
}

// checkDevices refreshes the mic list, says when a mic was plugged in, and keeps an active recording on a live mic:
// the current one while it is listed and its stream alive, otherwise the best one left.
func (r *Recorder) checkDevices(ctx context.Context) {
	r.mu.Lock()
	defer r.mu.Unlock()
	before := r.devices
	// A failed listing still goes on with the last good list: a dead stream left open would wake the loop again at
	// once, forever.
	if r.refreshDevices(ctx) {
		if added := domain.NewlyConnected(before, r.devices); len(added) > 0 {
			r.notice = Notice{Kind: NoticeMicConnected, Mic: added[0].Name()}
		}
	}
	if r.session.State() == domain.Idle || (r.streamAlive() && r.listed(r.micID)) {
		return
	}
	previous := nameOf(before, r.micID)
	current := r.micID
	if !r.listed(current) {
		current = ""
	}
	r.closeStream(ctx)
	r.reopen(ctx, domain.ChooseMic(r.devices, current, domain.MicID(r.settings.MicID)), previous)
}

// reopen opens the chosen mic for an active recording, or waits for one. previous names the mic that was lost,
// so a fallback can say what it replaced.
func (r *Recorder) reopen(ctx context.Context, choice domain.Choice, previous string) {
	now := r.deps.Clock.Now()
	if !r.openMic(ctx, choice) {
		if r.session.State() == domain.Recording {
			r.session.MicLost(now)
			r.flush(ctx)
		}
		r.notice = Notice{Kind: NoticeNoMic}
		return
	}
	r.session.MicBack(now)
	if mic, _ := choice.Mic(); choice.Reason() == domain.FellBackToDefault && previous != "" {
		r.notice = Notice{Kind: NoticeMicFallback, Mic: mic.Name(), PreviousMic: previous}
	}
}

func (r *Recorder) streamAlive() bool {
	if r.stream == nil {
		return false
	}
	select {
	case <-r.stream.Lost():
		return false
	default:
		return true
	}
}

func (r *Recorder) listed(id domain.MicID) bool {
	return id != "" && slices.ContainsFunc(r.devices, func(m domain.Mic) bool { return m.ID() == id })
}

func nameOf(devices []domain.Mic, id domain.MicID) string {
	if i := slices.IndexFunc(devices, func(m domain.Mic) bool { return m.ID() == id }); i >= 0 && id != "" {
		return devices[i].Name()
	}
	return ""
}
