# san_piano

Read what is played on a MIDI piano or keyboard from Go: every key pressed
and released, with its note name and velocity, and the pedals.

Module `github.com/wargasipil/san_piano`, Go 1.26, standard library only, no
cgo. Windows only for now: it uses WinMM (`winmm.dll`), which the Windows MIDI
Services of Windows 11 still serve, so USB instruments work as plugged in. On
other systems `Devices` and `Open` return `ErrUnsupported`.

## The tool

```powershell
pwsh build.ps1                      # vet, test, build bin/san_piano.exe
bin\san_piano.exe device list       # 0  General MIDI
bin\san_piano.exe debug             # show what is pressed on the first input
bin\san_piano.exe debug -device 1   # or pick it by number, or part of the name
bin\san_piano.exe debug -f debug.txt   # and save the lines to debug.txt too
```

```
== listening to General MIDI (input 0); press something, Ctrl+C to stop
    2.155s  90 4D 63  F5    77  on   velocity  99
    2.267s  80 4D 40  F5    77  off
    3.020s  B0 40 7F  sustain pedal down (127)
```

Each line is the time since the input was opened, the raw MIDI bytes, and
what they mean. Clock and active-sensing messages, which some instruments send
several times a second, are not shown.

With `-f`, the same lines are written to the file as they appear, so it is
complete even if the window is closed instead of stopped with Ctrl+C. An
existing file is replaced. The `==` status lines are not saved.

## The package

```go
package main

import (
	"fmt"
	"log"

	san_piano "github.com/wargasipil/san_piano"
)

func main() {
	d, err := san_piano.Find("") // the first input; or "1", or part of a name
	if err != nil {
		log.Fatal(err) // san_piano.ErrNoDevice: not plugged in or switched off
	}
	in, err := san_piano.Open(d.ID)
	if err != nil {
		log.Fatal(err) // e.g. another program holds the input
	}
	defer in.Close()

	for ev := range in.Events() { // closed by Close
		switch ev.Kind {
		case san_piano.NoteOn:
			fmt.Println("press", ev.Note, ev.Velocity) // press C4 87
		case san_piano.NoteOff:
			fmt.Println("release", ev.Note)
		case san_piano.ControlChange:
			if ev.Controller == san_piano.SustainPedal {
				fmt.Println("sustain", ev.Value >= 64)
			}
		}
	}
}
```

- `Note` 60 is middle C, which `String` calls `C4` (scientific pitch; Yamaha
  calls it C3). An 88-key piano runs from `A0` (21) to `C8` (108).
- A note-on with velocity 0, which many keyboards send to release a key,
  arrives as `NoteOff`.
- `Events` buffers 1024 events; a reader that stops reading loses the rest.
- `Parse` decodes a raw three-byte message, for use without a device.
