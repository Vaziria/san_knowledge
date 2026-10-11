// Package san_piano reads what is played on a MIDI piano or keyboard
// connected to the computer: every key pressed and released, with its note
// name and velocity, and the pedals.
//
//	in, err := san_piano.Open(0)
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer in.Close()
//	for ev := range in.Events() {
//		if ev.Kind == san_piano.NoteOn {
//			fmt.Println(ev.Note, ev.Velocity) // C4 87
//		}
//	}
//
// It uses the operating system's MIDI API directly, with no cgo and no
// dependencies. Only Windows is supported for now (WinMM, which the Windows
// MIDI Services of Windows 11 still serve); elsewhere Devices and Open return
// ErrUnsupported.
package san_piano

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
)

var (
	// ErrNoDevice reports that no MIDI input matches, or that there is none
	// at all: the instrument is not plugged in, or is switched off.
	ErrNoDevice = errors.New("san_piano: no MIDI input device")

	// ErrUnsupported is returned on operating systems this package has no
	// MIDI backend for yet.
	ErrUnsupported = errors.New("san_piano: MIDI input is only supported on Windows")
)

// eventBuffer is how many events an Input holds for a reader that has not
// caught up. Playing never comes close; a reader that stops reading loses the
// events beyond it.
const eventBuffer = 1024

// Device is a MIDI input: a piano, a keyboard, or anything else that sends
// MIDI to the computer.
type Device struct {
	// ID is the input's number for Open. Numbers can change when devices are
	// plugged in or out.
	ID   int
	Name string
}

// Devices lists the MIDI inputs, in ID order. An empty list with no error
// means none is connected.
func Devices() ([]Device, error) {
	return devices()
}

// Find picks a MIDI input by its ID ("1") or by part of its name ("piano"),
// ignoring case. An empty query picks the first input.
func Find(query string) (Device, error) {
	list, err := devices()
	if err != nil {
		return Device{}, err
	}
	id, err := strconv.Atoi(query)
	isID := err == nil
	for _, d := range list {
		if query == "" || (isID && d.ID == id) ||
			(!isID && strings.Contains(strings.ToLower(d.Name), strings.ToLower(query))) {
			return d, nil
		}
	}
	if query == "" {
		return Device{}, ErrNoDevice
	}
	return Device{}, fmt.Errorf("%w matching %q", ErrNoDevice, query)
}

// Open starts listening to the MIDI input with the given ID. Close it when
// done: on Windows another program cannot always use the input while it is
// open.
func Open(id int) (*Input, error) {
	list, err := devices()
	if err != nil {
		return nil, err
	}
	for _, d := range list {
		if d.ID == id {
			return open(d)
		}
	}
	return nil, fmt.Errorf("%w with ID %d", ErrNoDevice, id)
}

// Input is an open MIDI input. Its methods are safe for concurrent use.
type Input struct {
	Device Device

	events    chan Event
	closeOnce sync.Once
	closeErr  error
	sys       platformInput
}

// Events delivers the messages the instrument sends, in order, as they
// arrive. The channel is closed by Close.
func (in *Input) Events() <-chan Event {
	return in.events
}

// Close stops listening and closes the Events channel. Calling it again does
// nothing and returns the first call's error.
func (in *Input) Close() error {
	in.closeOnce.Do(func() { in.closeErr = in.close() })
	return in.closeErr
}
