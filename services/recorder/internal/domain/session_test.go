package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/tienduc120/voice-recorder/services/recorder/internal/domain"
)

func at(sec int) time.Time {
	return time.Date(2026, time.November, 9, 10, 30, 0, 0, time.UTC).Add(time.Duration(sec) * time.Second)
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestRecordedTimeCountsOnlyTimeSpentRecording(t *testing.T) {
	t.Parallel()

	var s domain.Session
	must(t, s.Start(at(0), true))
	must(t, s.Pause(at(10)))
	must(t, s.Resume(at(25), true))
	s.MicLost(at(30))
	s.MicBack(at(40))
	must(t, s.Stop(at(50)))

	if got := s.Recorded(at(99)); got != 25*time.Second {
		t.Errorf("recorded = %v, want 25s: paused time and time without a mic must not count", got)
	}
}

func TestStartWithoutAMicWaitsForOne(t *testing.T) {
	t.Parallel()

	var s domain.Session
	must(t, s.Start(at(0), false))

	if s.State() != domain.WaitingForMic || s.Recorded(at(5)) != 0 {
		t.Errorf("state %v, recorded %v: want waiting for mic with nothing recorded", s.State(), s.Recorded(at(5)))
	}
}

func TestResumeWithoutAMicWaitsForOne(t *testing.T) {
	t.Parallel()

	var s domain.Session
	must(t, s.Start(at(0), true))
	must(t, s.Pause(at(1)))
	must(t, s.Resume(at(2), false))

	if s.State() != domain.WaitingForMic {
		t.Errorf("state = %v, want waiting for mic", s.State())
	}
}

func TestStartingAgainResetsTheRecordedTime(t *testing.T) {
	t.Parallel()

	var s domain.Session
	must(t, s.Start(at(0), true))
	must(t, s.Stop(at(10)))
	must(t, s.Start(at(20), true))

	if got := s.Recorded(at(25)); got != 5*time.Second {
		t.Errorf("recorded = %v, want 5s: a new recording starts from zero", got)
	}
}

func TestStopKeepsTheRecordedTimeForDisplay(t *testing.T) {
	t.Parallel()

	var s domain.Session
	must(t, s.Start(at(0), true))
	must(t, s.Stop(at(10)))

	if got := s.Recorded(at(99)); got != 10*time.Second {
		t.Errorf("recorded = %v, want 10s: the timer keeps showing the last recording", got)
	}
}

func TestPausingWhileWaitingForAMicPauses(t *testing.T) {
	t.Parallel()

	var s domain.Session
	must(t, s.Start(at(0), false))
	must(t, s.Pause(at(1)))
	if s.State() != domain.Paused {
		t.Fatalf("state = %v, want paused", s.State())
	}

	must(t, s.Resume(at(2), true))

	if s.State() != domain.Recording {
		t.Errorf("state = %v, want recording once resumed with a mic", s.State())
	}
}

func TestMicEventsOnlyMoveBetweenRecordingAndWaiting(t *testing.T) {
	t.Parallel()

	var paused domain.Session
	must(t, paused.Start(at(0), true))
	must(t, paused.Pause(at(1)))
	paused.MicLost(at(2))

	var idle domain.Session
	idle.MicBack(at(0))

	var recording domain.Session
	must(t, recording.Start(at(0), true))
	recording.MicBack(at(1))

	if paused.State() != domain.Paused || idle.State() != domain.Idle || recording.State() != domain.Recording {
		t.Errorf("paused %v, idle %v, recording %v: a mic event must not change those states",
			paused.State(), idle.State(), recording.State())
	}
}

func TestCommandsOutsideTheirStatesAreRefused(t *testing.T) {
	t.Parallel()

	idle := func(*testing.T, *domain.Session) {}
	recording := func(t *testing.T, s *domain.Session) { t.Helper(); must(t, s.Start(at(0), true)) }
	paused := func(t *testing.T, s *domain.Session) { t.Helper(); recording(t, s); must(t, s.Pause(at(1))) }
	waiting := func(t *testing.T, s *domain.Session) { t.Helper(); must(t, s.Start(at(0), false)) }

	start := func(s *domain.Session) error { return s.Start(at(5), true) }
	pause := func(s *domain.Session) error { return s.Pause(at(5)) }
	resume := func(s *domain.Session) error { return s.Resume(at(5), true) }
	stop := func(s *domain.Session) error { return s.Stop(at(5)) }

	for name, tc := range map[string]struct {
		setup func(*testing.T, *domain.Session)
		act   func(*domain.Session) error
	}{
		"start while recording":       {recording, start},
		"start while paused":          {paused, start},
		"start while waiting for mic": {waiting, start},
		"pause while idle":            {idle, pause},
		"pause while paused":          {paused, pause},
		"resume while idle":           {idle, resume},
		"resume while recording":      {recording, resume},
		"resume while waiting":        {waiting, resume},
		"stop while idle":             {idle, stop},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var s domain.Session
			tc.setup(t, &s)
			before := s.State()

			err := tc.act(&s)

			if !errors.Is(err, domain.ErrWrongState) {
				t.Errorf("err = %v, want ErrWrongState", err)
			}
			if s.State() != before {
				t.Errorf("state = %v, want %v: a refused command changes nothing", s.State(), before)
			}
		})
	}
}

func TestStatesHaveReadableNames(t *testing.T) {
	t.Parallel()

	got := []string{
		domain.Idle.String(), domain.Recording.String(), domain.Paused.String(), domain.WaitingForMic.String(),
	}
	want := []string{"idle", "recording", "paused", "waiting for mic"}

	for i := range want {
		if got[i] != want[i] {
			t.Errorf("names = %q, want %q", got, want)
			break
		}
	}
}
