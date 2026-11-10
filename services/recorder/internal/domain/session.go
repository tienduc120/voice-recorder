// Package domain holds the rules of a recording: its states, its file name, and which mic it uses.
package domain

import (
	"errors"
	"fmt"
	"time"
)

// State is where a recording is in its life.
type State int

const (
	Idle State = iota
	Recording
	Paused
	// WaitingForMic is a pause the recorder took by itself because no mic is connected.
	WaitingForMic
)

func (s State) String() string {
	switch s {
	case Idle:
		return "idle"
	case Recording:
		return "recording"
	case Paused:
		return "paused"
	case WaitingForMic:
		return "waiting for mic"
	}
	return fmt.Sprintf("state(%d)", int(s))
}

var ErrWrongState = errors.New("not possible in this state")

// Session is one recording's life: its state and how long it has actually recorded. Time arrives as values.
type Session struct {
	state    State
	recorded time.Duration
	// since is when the open recording interval began; it only means something while recording.
	since time.Time
}

func (s *Session) Start(now time.Time, micAvailable bool) error {
	if s.state != Idle {
		return s.refuse("start")
	}
	s.recorded = 0
	s.enter(now, recordingIf(micAvailable))
	return nil
}

func (s *Session) Pause(now time.Time) error {
	if s.state != Recording && s.state != WaitingForMic {
		return s.refuse("pause")
	}
	s.enter(now, Paused)
	return nil
}

func (s *Session) Resume(now time.Time, micAvailable bool) error {
	if s.state != Paused {
		return s.refuse("resume")
	}
	s.enter(now, recordingIf(micAvailable))
	return nil
}

// Stop keeps the recorded time, so the timer still shows the last recording until the next Start.
func (s *Session) Stop(now time.Time) error {
	if s.state == Idle {
		return s.refuse("stop")
	}
	s.enter(now, Idle)
	return nil
}

// MicLost matters only while recording: a paused session stays paused, and the owner's pause wins.
func (s *Session) MicLost(now time.Time) {
	if s.state == Recording {
		s.enter(now, WaitingForMic)
	}
}

// MicBack resumes only a pause the recorder took by itself, never one the owner chose.
func (s *Session) MicBack(now time.Time) {
	if s.state == WaitingForMic {
		s.enter(now, Recording)
	}
}

func (s *Session) State() State { return s.state }

func (s *Session) Recorded(now time.Time) time.Duration {
	if s.state == Recording {
		return s.recorded + now.Sub(s.since)
	}
	return s.recorded
}

func (s *Session) enter(now time.Time, next State) {
	if s.state == Recording {
		s.recorded += now.Sub(s.since)
	}
	if next == Recording {
		s.since = now
	}
	s.state = next
}

func (s *Session) refuse(verb string) error {
	return fmt.Errorf("%w: cannot %s while %s", ErrWrongState, verb, s.state)
}

func recordingIf(micAvailable bool) State {
	if micAvailable {
		return Recording
	}
	return WaitingForMic
}
