//go:build windows

package san_piano

import (
	"fmt"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

// The WinMM MIDI input API. Windows 11's MIDI Services still serve it, so it
// reaches USB and Bluetooth instruments alike, and it needs nothing beyond
// winmm.dll, which every Windows has.
var (
	winmm                   = syscall.NewLazyDLL("winmm.dll")
	procMidiInGetNumDevs    = winmm.NewProc("midiInGetNumDevs")
	procMidiInGetDevCapsW   = winmm.NewProc("midiInGetDevCapsW")
	procMidiInGetErrorTextW = winmm.NewProc("midiInGetErrorTextW")
	procMidiInOpen          = winmm.NewProc("midiInOpen")
	procMidiInStart         = winmm.NewProc("midiInStart")
	procMidiInStop          = winmm.NewProc("midiInStop")
	procMidiInReset         = winmm.NewProc("midiInReset")
	procMidiInClose         = winmm.NewProc("midiInClose")
)

const (
	callbackFunction = 0x00030000 // CALLBACK_FUNCTION
	mimData          = 0x3C3      // MIM_DATA: a short message arrived
	maxErrorLength   = 256        // MAXERRORLENGTH
)

// midiInCapsW is MIDIINCAPSW.
type midiInCapsW struct {
	Mid, Pid      uint16
	DriverVersion uint32
	Pname         [32]uint16 // MAXPNAMELEN
	Support       uint32
}

type platformInput struct {
	handle uintptr // HMIDIIN
	id     uintptr // key in inputs, handed to WinMM as dwInstance
}

var (
	// callback is shared by every Input: a process can only create a limited
	// number of callbacks, and they are never freed. WinMM calls it on a
	// thread of its own, so it must not block.
	callback = syscall.NewCallback(midiInProc)

	// inputs maps the dwInstance WinMM passes the callback to the Input it
	// was opened for. The lock also keeps the callback from sending on a
	// channel Close is closing.
	inputsMu  sync.Mutex
	inputs    = map[uintptr]*Input{}
	nextInput uintptr
)

func midiInProc(handle, msg, instance, param1, param2 uintptr) uintptr {
	if msg != mimData {
		return 0
	}
	// param1 packs the message (status in the low byte; WinMM always gives
	// the status, never running status), param2 the time in milliseconds
	// since midiInStart.
	ev := Parse(byte(param1), byte(param1>>8), byte(param1>>16))
	ev.Time = time.Duration(param2) * time.Millisecond

	inputsMu.Lock()
	defer inputsMu.Unlock()
	if in := inputs[instance]; in != nil {
		select {
		case in.events <- ev:
		default: // the reader is not reading; see eventBuffer
		}
	}
	return 0
}

func devices() ([]Device, error) {
	n, _, _ := procMidiInGetNumDevs.Call()
	list := make([]Device, 0, n)
	for i := range int(n) {
		var caps midiInCapsW
		r, _, _ := procMidiInGetDevCapsW.Call(uintptr(i), uintptr(unsafe.Pointer(&caps)), unsafe.Sizeof(caps))
		if r != 0 {
			return nil, mmError("reading MIDI input "+fmt.Sprint(i), r)
		}
		list = append(list, Device{ID: i, Name: syscall.UTF16ToString(caps.Pname[:])})
	}
	return list, nil
}

func open(d Device) (*Input, error) {
	in := &Input{Device: d, events: make(chan Event, eventBuffer)}

	inputsMu.Lock()
	nextInput++
	in.sys.id = nextInput
	inputs[in.sys.id] = in
	inputsMu.Unlock()

	r, _, _ := procMidiInOpen.Call(uintptr(unsafe.Pointer(&in.sys.handle)), uintptr(d.ID),
		callback, in.sys.id, callbackFunction)
	if r != 0 {
		forget(in)
		return nil, mmError("opening "+d.Name, r)
	}
	if r, _, _ := procMidiInStart.Call(in.sys.handle); r != 0 {
		procMidiInClose.Call(in.sys.handle)
		forget(in)
		return nil, mmError("starting "+d.Name, r)
	}
	return in, nil
}

func (in *Input) close() error {
	procMidiInStop.Call(in.sys.handle)
	procMidiInReset.Call(in.sys.handle)
	// Once midiInClose returns, WinMM sends no more messages.
	r, _, _ := procMidiInClose.Call(in.sys.handle)
	forget(in)
	if r != 0 {
		return mmError("closing "+in.Device.Name, r)
	}
	return nil
}

// forget unregisters in from the callback and closes its Events channel.
func forget(in *Input) {
	inputsMu.Lock()
	defer inputsMu.Unlock()
	delete(inputs, in.sys.id)
	close(in.events)
}

// mmError turns a WinMM MMRESULT into an error with Windows' own text, which
// for the common case, another program holding the input, says so.
func mmError(doing string, code uintptr) error {
	var buf [maxErrorLength]uint16
	text := fmt.Sprintf("MMRESULT %d", code)
	if r, _, _ := procMidiInGetErrorTextW.Call(code, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf))); r == 0 {
		text = syscall.UTF16ToString(buf[:])
	}
	return fmt.Errorf("san_piano: %s: %s", doing, text)
}
