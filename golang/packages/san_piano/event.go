package san_piano

import (
	"fmt"
	"time"
)

// Note is a MIDI note number, 0 to 127. Middle C is 60.
type Note uint8

var noteNames = [12]string{"C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B"}

// String names the note in scientific pitch notation: 60 (middle C) is "C4",
// 61 is "C#4", and an 88-key piano runs from 21, "A0", to 108, "C8". Some
// makers, Yamaha among them, call 60 "C3"; this package does not.
func (n Note) String() string {
	return fmt.Sprintf("%s%d", noteNames[n%12], int(n)/12-1)
}

// Kind is what an Event is.
type Kind uint8

const (
	// NoteOff is a key released. A NoteOn with velocity 0, which many
	// keyboards send instead, is reported as a NoteOff too.
	NoteOff Kind = iota + 1
	// NoteOn is a key pressed.
	NoteOn
	// PolyPressure is aftertouch on one key that is held down.
	PolyPressure
	// ControlChange is a pedal, knob, slider or switch.
	ControlChange
	// ProgramChange is a change of sound (instrument).
	ProgramChange
	// ChannelPressure is aftertouch on the whole keyboard.
	ChannelPressure
	// PitchBend is the pitch bend wheel or lever.
	PitchBend
	// System is anything else: clock, active sensing, song position and the
	// like. They have no channel.
	System
)

var kindNames = [...]string{
	NoteOff:         "note off",
	NoteOn:          "note on",
	PolyPressure:    "poly pressure",
	ControlChange:   "control change",
	ProgramChange:   "program change",
	ChannelPressure: "channel pressure",
	PitchBend:       "pitch bend",
	System:          "system",
}

func (k Kind) String() string {
	if int(k) < len(kindNames) && kindNames[k] != "" {
		return kindNames[k]
	}
	return fmt.Sprintf("Kind(%d)", k)
}

// Controller numbers a piano commonly sends as a ControlChange.
const (
	SustainPedal   = 64 // the right pedal
	SostenutoPedal = 66 // the middle pedal
	SoftPedal      = 67 // the left pedal
)

// Event is one MIDI message from the instrument.
type Event struct {
	Kind Kind
	// Channel is the MIDI channel, 0 to 15; instruments show it as 1 to 16.
	// System events leave it 0.
	Channel uint8
	// Note is the key, for NoteOn, NoteOff and PolyPressure.
	Note Note
	// Velocity is how hard the key was struck, 1 to 127, for NoteOn, or
	// released, for NoteOff (most keyboards send 0 or 64 there).
	Velocity uint8
	// Controller is the controller number of a ControlChange, such as
	// SustainPedal.
	Controller uint8
	// Value is the value of a ControlChange (0 to 127; a pedal is down from
	// 64), the program of a ProgramChange (0 to 127), the pressure of
	// PolyPressure and ChannelPressure (0 to 127), or the amount of a
	// PitchBend (-8192 to 8191, 0 in the middle).
	Value int
	// Time is when the message arrived, counted from when the input was
	// opened. The driver gives it in milliseconds.
	Time time.Duration
	// Raw is the message as received: the status byte and two data bytes,
	// of which a message may use fewer.
	Raw [3]byte
}

// Bytes returns the bytes of Raw the message uses: three for a note, two for
// a program change, one for most system messages.
func (ev Event) Bytes() []byte {
	status := ev.Raw[0]
	n := 1
	switch {
	case status < 0x80:
	case status < 0xC0, status >= 0xE0 && status < 0xF0, status == 0xF2:
		n = 3
	case status < 0xE0, status == 0xF1, status == 0xF3:
		n = 2
	}
	return ev.Raw[:n]
}

// Parse decodes one short MIDI message given as its status byte and data
// bytes. Time is left 0.
func Parse(status, data1, data2 byte) Event {
	ev := Event{Raw: [3]byte{status, data1, data2}, Kind: System}
	if status < 0x80 || status >= 0xF0 {
		return ev
	}
	data1, data2 = data1&0x7F, data2&0x7F
	ev.Channel = status & 0x0F
	switch status & 0xF0 {
	case 0x80:
		ev.Kind, ev.Note, ev.Velocity = NoteOff, Note(data1), data2
	case 0x90:
		ev.Kind, ev.Note, ev.Velocity = NoteOn, Note(data1), data2
		if data2 == 0 {
			ev.Kind = NoteOff
		}
	case 0xA0:
		ev.Kind, ev.Note, ev.Value = PolyPressure, Note(data1), int(data2)
	case 0xB0:
		ev.Kind, ev.Controller, ev.Value = ControlChange, data1, int(data2)
	case 0xC0:
		ev.Kind, ev.Value = ProgramChange, int(data1)
	case 0xD0:
		ev.Kind, ev.Value = ChannelPressure, int(data1)
	case 0xE0:
		ev.Kind, ev.Value = PitchBend, int(data2)<<7|int(data1)-8192
	}
	return ev
}
