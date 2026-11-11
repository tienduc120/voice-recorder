package application_test

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/tienduc120/voice-recorder/services/recorder/internal/application"
)

func TestStartRecordsFromTheLastUsedMicIntoANewFile(t *testing.T) {
	t.Parallel()

	d := newDeps(meetingClock())
	rec := application.NewRecorder(t.Context(), d.Deps())

	st := mustStatus(t)(rec.Start(t.Context(), application.StartCommand{Title: "sprint planning"}))

	if got := d.files.createdFolders(); !reflect.DeepEqual(got, []string{recFolder}) {
		t.Errorf("created in %q, want one file in %q", got, recFolder)
	}
	if got := d.files.names[0](1); got != "2026-11-09_1030_sprint-planning.ogg" {
		t.Errorf("first name = %q, want the start time and the cleaned title", got)
	}
	if d.files.opts[0] != (application.AudioOptions{NoiseCleanup: true}) {
		t.Errorf("options = %+v, want the remembered noise cleanup", d.files.opts[0])
	}
	if got := d.capture.openedIDs(); !reflect.DeepEqual(got, []string{"usb"}) {
		t.Errorf("opened %q, want the last-used mic", got)
	}
	if holds := d.awake.counts().holds; st.State != application.StateRecording || holds != 1 {
		t.Errorf("state %q, sleep holds %d: want recording with the laptop kept awake", st.State, holds)
	}
	if st.File.Name != "2026-11-09_1030_sprint-planning.ogg" || st.MicID != "usb" {
		t.Errorf("status file %+v, mic %q: want the new file and the mic in use", st.File, st.MicID)
	}
}

func TestStartRefusesAFolderItCannotWriteTo(t *testing.T) {
	t.Parallel()

	d := newDeps(meetingClock())
	d.files.checkErr = fmt.Errorf("%w: gone", application.ErrFolderNotWritable)
	rec := application.NewRecorder(t.Context(), d.Deps())

	_, err := rec.Start(t.Context(), application.StartCommand{})

	if !errors.Is(err, application.ErrInvalidCommand) || !errors.Is(err, application.ErrFolderNotWritable) {
		t.Errorf("err = %v, want ErrInvalidCommand wrapping ErrFolderNotWritable", err)
	}
	if len(d.files.createdFolders()) != 0 || len(d.capture.openedIDs()) != 0 {
		t.Error("nothing may be created or opened when the folder cannot be written to")
	}
	if st := rec.Status(t.Context()); st.State != application.StateIdle || st.FolderProblem == "" {
		t.Errorf("status %+v: want idle with the folder problem shown", st)
	}
}

func TestStartWhileRecordingIsAConflictThatKeepsTheFile(t *testing.T) {
	t.Parallel()

	d := newDeps(meetingClock())
	rec := application.NewRecorder(t.Context(), d.Deps())
	mustStatus(t)(rec.Start(t.Context(), application.StartCommand{}))

	_, err := rec.Start(t.Context(), application.StartCommand{})

	if !errors.Is(err, application.ErrConflict) {
		t.Errorf("err = %v, want ErrConflict: a second tab must not start a second recording", err)
	}
	if len(d.files.createdFolders()) != 1 || d.files.first().counts().closes != 0 {
		t.Error("the running recording must keep its one open file")
	}
}

func TestStartWithNoMicWaitsForOneAndSaysSo(t *testing.T) {
	t.Parallel()

	d := newDeps(meetingClock())
	d.mics.set()
	rec := application.NewRecorder(t.Context(), d.Deps())

	st := mustStatus(t)(rec.Start(t.Context(), application.StartCommand{}))

	if st.State != application.StateWaitingForMic || st.Notice.Kind != application.NoticeNoMic {
		t.Errorf("state %q, notice %+v: want waiting for a mic, and saying so", st.State, st.Notice)
	}
	if len(d.files.createdFolders()) != 1 {
		t.Error("the file is created at Start even without a mic: one recording is one file")
	}
}

func TestPauseFlushesAndResumeContinuesTheSameFile(t *testing.T) {
	t.Parallel()

	d := newDeps(meetingClock())
	rec := application.NewRecorder(t.Context(), d.Deps())
	mustStatus(t)(rec.Start(t.Context(), application.StartCommand{}))

	paused := mustStatus(t)(rec.Pause(t.Context()))
	flushes := d.files.first().counts().flushes
	if paused.State != application.StatePaused || flushes != 1 || d.capture.firstStream().closeCount() != 0 {
		t.Errorf("state %q, flushes %d: want paused, flushed, and the mic still open", paused.State, flushes)
	}

	resumed := mustStatus(t)(rec.Resume(t.Context()))

	if resumed.State != application.StateRecording || len(d.files.createdFolders()) != 1 {
		t.Errorf("state %q: want recording again into the same file", resumed.State)
	}
	if got := d.capture.openedIDs(); !reflect.DeepEqual(got, []string{"usb"}) {
		t.Errorf("opened %q: the open mic is reused, not reopened", got)
	}
}

func TestRecordedTimeLeavesOutPauses(t *testing.T) {
	t.Parallel()

	clock := meetingClock()
	d := newDeps(clock)
	rec := application.NewRecorder(t.Context(), d.Deps())
	mustStatus(t)(rec.Start(t.Context(), application.StartCommand{}))
	clock.Advance(10 * time.Second)
	mustStatus(t)(rec.Pause(t.Context()))
	clock.Advance(15 * time.Second)
	mustStatus(t)(rec.Resume(t.Context()))
	clock.Advance(5 * time.Second)

	if got := rec.Status(t.Context()).Recorded; got != 15*time.Second {
		t.Errorf("recorded = %v, want 15s: paused time does not count", got)
	}
}

func TestStopFinalisesTheFileAndLetsGoOfTheMicAndSleep(t *testing.T) {
	t.Parallel()

	d := newDeps(meetingClock())
	rec := application.NewRecorder(t.Context(), d.Deps())
	mustStatus(t)(rec.Start(t.Context(), application.StartCommand{Title: "retro"}))

	st := mustStatus(t)(rec.Stop(t.Context()))

	closes := d.files.first().counts().closes
	releases := d.awake.counts().releases
	if closes != 1 || d.capture.firstStream().closeCount() != 1 || releases != 1 {
		t.Errorf("file closes %d, mic closes %d, sleep releases %d: want 1 each",
			closes, d.capture.firstStream().closeCount(), releases)
	}
	want := application.FileInfo{Name: "2026-11-09_1030_retro.ogg", Size: fakeSize}
	if st.State != application.StateIdle || st.File != want {
		t.Errorf("state %q, file %+v: want idle showing %+v", st.State, st.File, want)
	}
}

func TestStopKeepsTheAudioStillQueued(t *testing.T) {
	t.Parallel()

	d := newDeps(meetingClock())
	rec := application.NewRecorder(t.Context(), d.Deps())
	mustStatus(t)(rec.Start(t.Context(), application.StartCommand{}))
	for range 3 {
		d.capture.send(0.1)
	}

	mustStatus(t)(rec.Stop(t.Context()))

	if writes := d.files.first().counts().writes; writes != 3 {
		t.Errorf("writes = %d, want 3: the last words before Stop belong in the file", writes)
	}
}

func TestAFailedFinaliseIsReportedButStillStops(t *testing.T) {
	t.Parallel()

	d := newDeps(meetingClock())
	rec := application.NewRecorder(t.Context(), d.Deps())
	mustStatus(t)(rec.Start(t.Context(), application.StartCommand{}))
	d.files.first().closeErr = fmt.Errorf("%w: x", application.ErrDiskFull)

	st, err := rec.Stop(t.Context())

	if err != nil || st.State != application.StateIdle || st.Notice.Kind != application.NoticeDiskFull {
		t.Errorf("err %v, state %q, notice %+v: want stopped, no error, and the disk-full notice",
			err, st.State, st.Notice)
	}
}

func TestChangeSettingsTakesAPathPastedWithQuotes(t *testing.T) {
	t.Parallel()

	d := newDeps(meetingClock())
	rec := application.NewRecorder(t.Context(), d.Deps())

	mustStatus(t)(rec.ChangeSettings(t.Context(), application.SettingsCommand{
		Folder: `  "C:\Users\me\Rec"  `, NoiseCleanup: false,
	}))

	if got := d.files.checked[len(d.files.checked)-1]; got != `C:\Users\me\Rec` {
		t.Errorf("checked %q, want the path without the quotes Explorer's Copy as path adds", got)
	}
	want := application.Settings{Folder: `C:\Users\me\Rec`, MicID: "usb", NoiseCleanup: false}
	if saves := d.settings.saves(); len(saves) == 0 || saves[len(saves)-1] != want {
		t.Errorf("saved %+v, want %+v", saves, want)
	}
}

func TestChangeSettingsRefusesAnUnwritableFolderWithoutSaving(t *testing.T) {
	t.Parallel()

	d := newDeps(meetingClock())
	rec := application.NewRecorder(t.Context(), d.Deps())
	d.files.checkErr = fmt.Errorf("%w: read-only", application.ErrFolderNotWritable)

	_, err := rec.ChangeSettings(t.Context(), application.SettingsCommand{Folder: `D:\locked`})

	if !errors.Is(err, application.ErrInvalidCommand) || !errors.Is(err, application.ErrFolderNotWritable) {
		t.Errorf("err = %v, want ErrInvalidCommand wrapping ErrFolderNotWritable", err)
	}
	if len(d.settings.saves()) != 0 {
		t.Error("a folder that failed its check must not be remembered")
	}
}

func TestChangeSettingsWaitsUntilTheRecordingStops(t *testing.T) {
	t.Parallel()

	d := newDeps(meetingClock())
	rec := application.NewRecorder(t.Context(), d.Deps())
	mustStatus(t)(rec.Start(t.Context(), application.StartCommand{}))
	before := len(d.settings.saves())

	_, err := rec.ChangeSettings(t.Context(), application.SettingsCommand{Folder: `D:\other`})

	if !errors.Is(err, application.ErrConflict) || len(d.settings.saves()) != before {
		t.Errorf("err = %v: want ErrConflict and nothing saved while recording", err)
	}
}

func TestNewRecorderCarriesOnWithDefaultsWhenSettingsAreUnreadable(t *testing.T) {
	t.Parallel()

	d := newDeps(meetingClock())
	d.settings.stored = application.Settings{Folder: `C:\Docs\Recordings`, NoiseCleanup: true}
	d.settings.loadErr = errors.New("corrupt")

	st := application.NewRecorder(t.Context(), d.Deps()).Status(t.Context())

	if st.Folder != `C:\Docs\Recordings` || !st.NoiseCleanup {
		t.Errorf("status %+v: want the store's defaults, not a failure to start", st)
	}
}

func TestBrowseReturnsThePickedFolderWithoutSavingIt(t *testing.T) {
	t.Parallel()

	d := newDeps(meetingClock())
	d.picker.folder, d.picker.chosen = `D:\Meetings`, true
	rec := application.NewRecorder(t.Context(), d.Deps())

	got, err := rec.Browse(t.Context())

	if err != nil || got != (application.BrowseResult{Folder: `D:\Meetings`, Chosen: true}) {
		t.Errorf("browse = %+v, %v; want the picked folder", got, err)
	}
	if d.picker.start != recFolder || len(d.settings.saves()) != 0 {
		t.Errorf("picker started at %q, saves %d: want the current folder and nothing saved",
			d.picker.start, len(d.settings.saves()))
	}
}

func TestOpenFolderShowsTheSaveFolderEvenWhileRecording(t *testing.T) {
	t.Parallel()

	d := newDeps(meetingClock())
	rec := application.NewRecorder(t.Context(), d.Deps())
	mustStatus(t)(rec.Start(t.Context(), application.StartCommand{}))

	if err := rec.OpenFolder(t.Context()); err != nil {
		t.Fatal(err)
	}

	if got := d.opener.folders(); !reflect.DeepEqual(got, []string{recFolder}) {
		t.Errorf("opened %q, want the save folder", got)
	}
}

func TestOpenFolderRefusesAFolderThatCannotBeUsed(t *testing.T) {
	t.Parallel()

	d := newDeps(meetingClock())
	d.files.checkErr = fmt.Errorf("%w: gone", application.ErrFolderNotWritable)
	rec := application.NewRecorder(t.Context(), d.Deps())

	err := rec.OpenFolder(t.Context())

	if !errors.Is(err, application.ErrInvalidCommand) {
		t.Errorf("err = %v, want ErrInvalidCommand", err)
	}
	if len(d.opener.folders()) != 0 {
		t.Error("a folder that cannot be used must not be opened")
	}
}

func TestStatusNamesTheFileARecordingWithoutATitleWouldGet(t *testing.T) {
	t.Parallel()

	clock := meetingClock()
	rec := application.NewRecorder(t.Context(), newDeps(clock).Deps())
	clock.Advance(time.Minute)

	if got := rec.Status(t.Context()).DefaultName; got != "2026-11-09_1031.ogg" {
		t.Errorf("default name = %q, want the current minute's name", got)
	}
}

func TestQuitWhileRecordingFinalisesBeforeQuitting(t *testing.T) {
	t.Parallel()

	d := newDeps(meetingClock())
	rec := application.NewRecorder(t.Context(), d.Deps())
	mustStatus(t)(rec.Start(t.Context(), application.StartCommand{}))

	if err := rec.Quit(t.Context()); err != nil {
		t.Fatal(err)
	}

	ev := d.ev.all()
	closed, quit := slices.Index(ev, "file.close"), slices.Index(ev, "quit")
	if closed < 0 || quit < 0 || closed > quit {
		t.Errorf("events %q: the file must be finalised before the app quits", ev)
	}
}
