package application_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/tienduc120/voice-recorder/services/recorder/internal/application"
)

// pollInterval mirrors the recorder's device poll, plus a step so the tick has fired.
const pollInterval = 2*time.Second + time.Millisecond

type running struct {
	cancel context.CancelFunc
	done   chan error
}

// startRun runs the frame loop until the test ends; the bubble requires it to have stopped by then.
func startRun(t *testing.T, rec *application.Recorder) *running {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	r := &running{cancel: cancel, done: make(chan error, 1)}
	go func() { r.done <- rec.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		<-r.done
	})
	return r
}

func (r *running) stillRunning() bool {
	select {
	case <-r.done:
		return false
	default:
		return true
	}
}

func tick() {
	<-time.After(pollInterval)
	synctest.Wait()
}

func startRecording(t *testing.T, d *deps) (*application.Recorder, *running) {
	t.Helper()
	rec := application.NewRecorder(t.Context(), d.Deps())
	mustStatus(t)(rec.Start(t.Context(), application.StartCommand{}))
	return rec, startRun(t, rec)
}

func TestRunWritesBlocksOnlyWhileRecording(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		d := newDeps(bubbleClock{})
		rec, _ := startRecording(t, d)

		d.capture.send(0.5)
		d.capture.send(0.5)
		synctest.Wait()
		mustStatus(t)(rec.Pause(t.Context()))
		d.capture.send(0.5)
		synctest.Wait()

		if writes := d.files.first().counts().writes; writes != 2 {
			t.Errorf("writes = %d, want 2: audio during a pause is not recorded", writes)
		}
		if level := rec.Status(t.Context()).Level; level < 0.89 || level > 0.91 {
			t.Errorf("level = %v, want about 0.9 for a peak of 0.5 (−6 dBFS on a 60 dB meter)", level)
		}
	})
}

func TestPluggingInAMicKeepsTheCurrentOneAndSaysSo(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		d := newDeps(bubbleClock{})
		d.mics.set(builtIn(true))
		d.settings.stored.MicID = "builtin"
		rec, _ := startRecording(t, d)

		d.mics.set(builtIn(false), usb(true))
		tick()

		if got := d.capture.openedIDs(); !reflect.DeepEqual(got, []string{"builtin"}) {
			t.Errorf("opened %q: a newly plugged-in mic must not take over, even as Windows' default", got)
		}
		want := application.Notice{Kind: application.NoticeMicConnected, Mic: usbName}
		if got := rec.Status(t.Context()).Notice; got != want {
			t.Errorf("notice = %+v, want %+v", got, want)
		}
	})
}

func TestUnpluggingTheMicInUseFallsBackToTheDefaultInTheSameFile(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		d := newDeps(bubbleClock{})
		rec, _ := startRecording(t, d)

		d.mics.set(builtIn(true))
		tick()

		if got := d.capture.openedIDs(); !reflect.DeepEqual(got, []string{"usb", "builtin"}) {
			t.Errorf("opened %q, want the USB mic and then the default", got)
		}
		if d.capture.firstStream().closeCount() != 1 || len(d.files.createdFolders()) != 1 {
			t.Error("the lost mic's stream is closed and the recording keeps its one file")
		}
		st := rec.Status(t.Context())
		want := application.Notice{Kind: application.NoticeMicFallback, Mic: builtInName, PreviousMic: usbName}
		if st.Notice != want || st.State != application.StateRecording {
			t.Errorf("state %q, notice %+v: want recording with %+v", st.State, st.Notice, want)
		}
	})
}

func TestALostStreamReopensTheSameMicWhenItIsStillThere(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		d := newDeps(bubbleClock{})
		rec, _ := startRecording(t, d)

		d.capture.firstStream().loseDevice()
		synctest.Wait()

		if got := d.capture.openedIDs(); !reflect.DeepEqual(got, []string{"usb", "usb"}) {
			t.Errorf("opened %q: after sleep and wake the same mic is reopened at once", got)
		}
		if st := rec.Status(t.Context()); st.State != application.StateRecording || len(d.files.createdFolders()) != 1 {
			t.Errorf("state %q: want still recording into the same file", st.State)
		}
	})
}

func TestLosingEveryMicWaitsThenCarriesOnWhenOneAppears(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		d := newDeps(bubbleClock{})
		rec, _ := startRecording(t, d)

		d.mics.set()
		tick()
		waiting := rec.Status(t.Context())
		<-time.After(10 * time.Second)
		later := rec.Status(t.Context())

		if waiting.State != application.StateWaitingForMic || waiting.Notice.Kind != application.NoticeNoMic {
			t.Errorf("state %q, notice %+v: want waiting for a mic, and saying so", waiting.State, waiting.Notice)
		}
		if d.files.first().counts().flushes < 1 || later.Recorded != waiting.Recorded {
			t.Errorf(
				"recorded %v then %v: the audio so far is flushed and the timer stops",
				waiting.Recorded,
				later.Recorded,
			)
		}

		d.mics.set(usb(true))
		tick()

		opened := d.capture.openedIDs()
		if st := rec.Status(t.Context()); st.State != application.StateRecording || opened[len(opened)-1] != "usb" {
			t.Errorf("state %q, opened %q: want recording again from the mic that came back", st.State, opened)
		}
		if len(d.files.createdFolders()) != 1 {
			t.Error("the recording keeps its one file across the gap")
		}
	})
}

func TestChoosingAMicMidRecordingSwitchesInTheSameFile(t *testing.T) {
	t.Parallel()

	d := newDeps(meetingClock())
	rec := application.NewRecorder(t.Context(), d.Deps())
	mustStatus(t)(rec.Start(t.Context(), application.StartCommand{}))

	mustStatus(t)(rec.ChooseMic(t.Context(), application.ChooseMicCommand{ID: "builtin"}))

	if got := d.capture.openedIDs(); !reflect.DeepEqual(got, []string{"usb", "builtin"}) {
		t.Errorf("opened %q, want a switch to the built-in mic", got)
	}
	if d.capture.firstStream().closeCount() != 1 || len(d.files.createdFolders()) != 1 {
		t.Error("the old stream is closed and the recording keeps its one file")
	}
	if saves := d.settings.saves(); len(saves) == 0 || saves[len(saves)-1].MicID != "builtin" {
		t.Errorf("saved %+v: the chosen mic is remembered", saves)
	}
}

func TestChoosingAMicThatWillNotStartKeepsRecordingFromTheOldOne(t *testing.T) {
	t.Parallel()

	d := newDeps(meetingClock())
	d.capture.refuse = "builtin"
	rec := application.NewRecorder(t.Context(), d.Deps())
	mustStatus(t)(rec.Start(t.Context(), application.StartCommand{}))

	st, err := rec.ChooseMic(t.Context(), application.ChooseMicCommand{ID: "builtin"})

	if !errors.Is(err, application.ErrConflict) || !strings.Contains(err.Error(), builtInName) {
		t.Errorf("err = %v, want ErrConflict naming the mic that would not start", err)
	}
	if got := d.capture.openedIDs(); !reflect.DeepEqual(got, []string{"usb", "builtin", "usb"}) {
		t.Errorf("opened %q, want the old mic back after the new one failed", got)
	}
	if st.State != application.StateRecording || st.MicID != "usb" || st.Notice.Kind != "" {
		t.Errorf("state %q, mic %q, notice %+v: want still recording from the old mic", st.State, st.MicID, st.Notice)
	}
	for _, saved := range d.settings.saves() {
		if saved.MicID == "builtin" {
			t.Error("a mic that would not start must not be remembered")
		}
	}
}

func TestALostStreamWhileMicsCannotBeListedDoesNotSpin(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		d := newDeps(bubbleClock{})
		rec, _ := startRecording(t, d)
		before := d.mics.fail(errors.New("the audio service is restarting"))

		d.capture.firstStream().loseDevice()
		synctest.Wait()

		if calls := d.mics.callCount() - before; calls > 1 {
			t.Errorf("listed the mics %d times at once: a dead stream must not spin the loop", calls)
		}
		if got := d.capture.openedIDs(); !reflect.DeepEqual(got, []string{"usb", "usb"}) {
			t.Errorf("opened %q: want the same mic reopened from the last good list", got)
		}
		if st := rec.Status(t.Context()); st.State != application.StateRecording {
			t.Errorf("state %q, want recording", st.State)
		}
	})
}

func TestChoosingAnUnknownMicIsAnInvalidCommand(t *testing.T) {
	t.Parallel()

	d := newDeps(meetingClock())
	rec := application.NewRecorder(t.Context(), d.Deps())

	_, err := rec.ChooseMic(t.Context(), application.ChooseMicCommand{ID: "gone"})

	if !errors.Is(err, application.ErrInvalidCommand) {
		t.Errorf("err = %v, want ErrInvalidCommand", err)
	}
}

func TestChoosingAMicWhileIdleOnlyRemembersIt(t *testing.T) {
	t.Parallel()

	d := newDeps(meetingClock())
	rec := application.NewRecorder(t.Context(), d.Deps())

	mustStatus(t)(rec.ChooseMic(t.Context(), application.ChooseMicCommand{ID: "builtin"}))

	if len(d.capture.openedIDs()) != 0 {
		t.Error("choosing a mic while idle must not open it")
	}
	if saves := d.settings.saves(); len(saves) == 0 || saves[len(saves)-1].MicID != "builtin" {
		t.Errorf("saved %+v: the chosen mic is remembered for the next Start", saves)
	}
}

func TestDiskFullStopsAndFinalisesTheRecording(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		d := newDeps(bubbleClock{})
		rec, run := startRecording(t, d)
		d.files.first().failWrites(fmt.Errorf("write: %w", application.ErrDiskFull))

		d.capture.send(0.1)
		synctest.Wait()

		st := rec.Status(t.Context())
		if st.State != application.StateIdle || st.Notice.Kind != application.NoticeDiskFull {
			t.Errorf("state %q, notice %+v: want stopped with the disk-full notice", st.State, st.Notice)
		}
		if d.files.first().counts().closes != 1 || d.awake.counts().releases != 1 || !run.stillRunning() {
			t.Error("the file is finalised, the laptop may sleep again, and the loop keeps running")
		}
	})
}

func TestAnyOtherWriteFailureAlsoStopsCleanly(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		d := newDeps(bubbleClock{})
		rec, run := startRecording(t, d)
		d.files.first().failWrites(errors.New("the network name is no longer available"))

		d.capture.send(0.1)
		synctest.Wait()

		st := rec.Status(t.Context())
		if st.State != application.StateIdle || st.Notice.Kind != application.NoticeWriteFailed {
			t.Errorf("state %q, notice %+v: want stopped with the write-failed notice", st.State, st.Notice)
		}
		if d.files.first().counts().closes != 1 || !run.stillRunning() {
			t.Error("the file is finalised as far as possible and the loop keeps running")
		}
	})
}

func TestCancellingRunFinalisesAnOpenRecording(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		d := newDeps(bubbleClock{})
		_, run := startRecording(t, d)

		run.cancel()

		if err := <-run.done; err != nil {
			t.Errorf("run = %v, want nil: being told to stop is a clean shutdown", err)
		}
		run.done <- nil // the cleanup waits on it too
		if d.files.first().counts().closes != 1 || d.awake.counts().releases != 1 {
			t.Error("an open recording is finalised and the sleep guard released when the app stops")
		}
	})
}

func TestTheMeterFallsSmoothlyAfterALoudBlock(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		d := newDeps(bubbleClock{})
		rec, _ := startRecording(t, d)

		d.capture.send(0.5)
		synctest.Wait()
		for range 10 {
			d.capture.send(0)
		}
		synctest.Wait()

		// 0.8997 for a 0.5 peak, less 10 blocks of 10 ms at 0.4 of the scale per second.
		if got := rec.Level(t.Context()); got < 0.855 || got > 0.865 {
			t.Errorf("level = %.4f, want about 0.86: a meter falls smoothly instead of dropping to zero", got)
		}
	})
}

func TestLevelIsZeroWhileIdle(t *testing.T) {
	t.Parallel()

	rec := application.NewRecorder(t.Context(), newDeps(meetingClock()).Deps())

	if got := rec.Level(t.Context()); got != 0 {
		t.Errorf("level = %v while idle, want 0", got)
	}
}
