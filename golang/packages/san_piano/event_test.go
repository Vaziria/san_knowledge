package san_piano

import "testing"

func TestNoteString(t *testing.T) {
	for n, want := range map[Note]string{
		0: "C-1", 21: "A0", 59: "B3", 60: "C4", 61: "C#4", 69: "A4", 108: "C8", 127: "G9",
	} {
		if got := n.String(); got != want {
			t.Errorf("Note(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestParse(t *testing.T) {
	tests := []struct {
		name           string
		status, d1, d2 byte
		kind           Kind
		channel        uint8
		note           Note
		velocity       uint8
		controller     uint8
		value          int
	}{
		{name: "note on", status: 0x90, d1: 60, d2: 87, kind: NoteOn, note: 60, velocity: 87},
		{name: "note on channel 10", status: 0x99, d1: 36, d2: 100, kind: NoteOn, channel: 9, note: 36, velocity: 100},
		{name: "note on velocity 0 is note off", status: 0x90, d1: 60, d2: 0, kind: NoteOff, note: 60},
		{name: "note off", status: 0x80, d1: 64, d2: 64, kind: NoteOff, note: 64, velocity: 64},
		{name: "sustain down", status: 0xB0, d1: 64, d2: 127, kind: ControlChange, controller: SustainPedal, value: 127},
		{name: "program", status: 0xC3, d1: 5, kind: ProgramChange, channel: 3, value: 5},
		{name: "poly pressure", status: 0xA0, d1: 60, d2: 40, kind: PolyPressure, note: 60, value: 40},
		{name: "channel pressure", status: 0xD0, d1: 33, kind: ChannelPressure, value: 33},
		{name: "pitch bend centre", status: 0xE0, d1: 0x00, d2: 0x40, kind: PitchBend, value: 0},
		{name: "pitch bend lowest", status: 0xE0, d1: 0x00, d2: 0x00, kind: PitchBend, value: -8192},
		{name: "pitch bend highest", status: 0xE0, d1: 0x7F, d2: 0x7F, kind: PitchBend, value: 8191},
		{name: "active sensing", status: 0xFE, kind: System},
		{name: "clock", status: 0xF8, kind: System},
		{name: "data byte without status", status: 0x3C, kind: System},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev := Parse(tt.status, tt.d1, tt.d2)
			if ev.Kind != tt.kind || ev.Channel != tt.channel || ev.Note != tt.note ||
				ev.Velocity != tt.velocity || ev.Controller != tt.controller || ev.Value != tt.value {
				t.Errorf("Parse(%#x, %d, %d) = %+v", tt.status, tt.d1, tt.d2, ev)
			}
			if ev.Raw != [3]byte{tt.status, tt.d1, tt.d2} {
				t.Errorf("Raw = %v", ev.Raw)
			}
		})
	}
}

func TestBytes(t *testing.T) {
	for status, want := range map[byte]int{
		0x90: 3, 0x80: 3, 0xA0: 3, 0xB5: 3, 0xE0: 3,
		0xC0: 2, 0xDF: 2, 0xF1: 2, 0xF3: 2,
		0xF2: 3,
		0xF6: 1, 0xF8: 1, 0xFA: 1, 0xFC: 1, 0xFE: 1, 0x3C: 1,
	} {
		if got := len(Parse(status, 1, 2).Bytes()); got != want {
			t.Errorf("%#x: %d bytes, want %d", status, got, want)
		}
	}
}

func TestKindString(t *testing.T) {
	if got := NoteOn.String(); got != "note on" {
		t.Errorf("NoteOn = %q", got)
	}
	if got := Kind(99).String(); got != "Kind(99)" {
		t.Errorf("Kind(99) = %q", got)
	}
}
