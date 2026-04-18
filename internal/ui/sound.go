package ui

import (
	"fmt"
	"io"
	"os"
)

// SoundType represents different sound events.
type SoundType int

const (
	SoundNodeDiscovered SoundType = iota
	SoundScanComplete
	SoundDeepScanStart
	SoundDeepScanComplete
	SoundPortOpen
	SoundError
)

// Sound plays terminal bell/tones if sound is enabled.
type Sound struct {
	enabled bool
	writer  io.Writer
}

// NewSound creates a new Sound controller.
func NewSound() *Sound {
	return &Sound{
		enabled: false,
		writer:  os.Stdout,
	}
}

// Toggle flips sound on/off and returns the new state.
func (s *Sound) Toggle() bool {
	s.enabled = !s.enabled
	return s.enabled
}

// IsEnabled returns whether sound is on.
func (s *Sound) IsEnabled() bool {
	return s.enabled
}

// Play emits a terminal sound for the given event type.
func (s *Sound) Play(st SoundType) {
	if !s.enabled {
		return
	}

	switch st {
	case SoundNodeDiscovered:
		// Single short beep
		s.bell(1)
	case SoundScanComplete:
		// Double beep
		s.bell(2)
	case SoundDeepScanStart:
		// Single beep
		s.bell(1)
	case SoundDeepScanComplete:
		// Triple beep
		s.bell(3)
	case SoundPortOpen:
		// Alert tone
		s.bell(1)
	case SoundError:
		// Single beep
		s.bell(1)
	}
}

func (s *Sound) bell(count int) {
	for i := 0; i < count; i++ {
		fmt.Fprint(s.writer, "\a")
	}
}
