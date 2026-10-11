// Command san_piano shows what is played on a MIDI piano or keyboard.
//
//	san_piano debug                   show what is pressed on the first MIDI input
//	san_piano debug -device 1         on input 1
//	san_piano debug -device piano     on the first input whose name has "piano"
//	san_piano debug -f debug.txt      and save the lines to debug.txt too
//	san_piano device list             list the MIDI inputs
//
// debug prints a line for every key pressed or released, pedal, control and
// button, with the raw MIDI bytes, until Ctrl+C. Status lines, starting with
// "==", go to stderr and are not saved.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"

	san_piano "github.com/wargasipil/san_piano"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		// The package's errors already say where they come from.
		msg := err.Error()
		if !strings.HasPrefix(msg, "san_piano: ") {
			msg = "san_piano: " + msg
		}
		fmt.Fprintln(os.Stderr, msg)
		os.Exit(1)
	}
}

const usage = `usage:
  san_piano debug [-device <number | name>] [-f <file>]   show what is pressed on the piano
  san_piano device list                                   list the MIDI inputs`

func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		fmt.Fprintln(stderr, usage)
		return nil
	}
	switch args[0] {
	case "debug":
		return runDebug(ctx, args[1:], stdout, stderr)
	case "device":
		return runDevice(args[1:], stdout, stderr)
	case "help", "-h", "-help", "--help":
		fmt.Fprintln(stderr, usage)
		return nil
	}
	fmt.Fprintln(stderr, usage)
	return fmt.Errorf("unknown command %q", args[0])
}

// runDebug runs "san_piano debug": it prints what is pressed until Ctrl+C.
func runDebug(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("san_piano debug", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		device = fs.String("device", "", "MIDI input to listen to: its number from 'san_piano device list', or part of its name (default: the first)")
		file   = fs.String("f", "", "also save the lines to this file, replacing what it held")
	)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: san_piano debug [-device <number | name>] [-f <file>]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return fmt.Errorf("debug: unexpected %q", fs.Arg(0))
	}

	d, err := san_piano.Find(*device)
	if errors.Is(err, san_piano.ErrNoDevice) {
		return fmt.Errorf("%w (see san_piano device list; is the instrument plugged in and switched on?)", err)
	}
	if err != nil {
		return err
	}
	in, err := san_piano.Open(d.ID)
	if err != nil {
		return err
	}
	defer in.Close()
	go func() {
		<-ctx.Done()
		in.Close()
	}()

	// The file is created only now, so a run that finds no piano leaves an
	// earlier file alone. It is unbuffered: every line is on disk as soon as
	// it is shown, even if the window is closed instead of stopped with Ctrl+C.
	out := stdout
	if *file != "" {
		f, err := os.Create(*file)
		if err != nil {
			return err
		}
		defer f.Close()
		out = io.MultiWriter(stdout, f)
	}

	fmt.Fprintf(stderr, "== listening to %s (input %d); press something, Ctrl+C to stop\n", d.Name, d.ID)
	if *file != "" {
		fmt.Fprintf(stderr, "== saving to %s\n", *file)
	}
	if err := show(in.Events(), out); err != nil {
		return err
	}
	return in.Close()
}

// show prints events to w, one line each, until events is closed.
func show(events <-chan san_piano.Event, w io.Writer) error {
	for ev := range events {
		if line := format(ev); line != "" {
			if _, err := fmt.Fprintln(w, line); err != nil {
				return err
			}
		}
	}
	return nil
}

// runDevice runs "san_piano device <command>".
func runDevice(args []string, stdout, stderr io.Writer) error {
	if len(args) == 1 && (args[0] == "-h" || args[0] == "-help" || args[0] == "--help") {
		fmt.Fprintln(stderr, usage)
		return nil
	}
	if len(args) != 1 || args[0] != "list" {
		fmt.Fprintln(stderr, usage)
		if len(args) == 0 {
			return errors.New("device: give a command, e.g. list")
		}
		return fmt.Errorf("device: unknown command %q", strings.Join(args, " "))
	}

	devices, err := san_piano.Devices()
	if err != nil {
		return err
	}
	if len(devices) == 0 {
		fmt.Fprintln(stderr, "== no MIDI inputs: is the instrument plugged in and switched on?")
	}
	for _, d := range devices {
		fmt.Fprintf(stdout, "%d  %s\n", d.ID, d.Name)
	}
	return nil
}

// systemNames names the system messages; the rest are shown as
// "system message".
var systemNames = map[byte]string{
	0xF1: "MIDI time code",
	0xF2: "song position",
	0xF3: "song select",
	0xF6: "tune request",
	0xFA: "start",
	0xFB: "continue",
	0xFC: "stop",
	0xFF: "reset",
}

// format renders an event as one line: the time, the raw bytes and what they
// mean. It returns "" for clock and active sensing, which some instruments
// send several times a second whether anything is pressed or not.
func format(ev san_piano.Event) string {
	var what string
	switch ev.Kind {
	case san_piano.NoteOn:
		what = fmt.Sprintf("%-4s %3d  on   velocity %3d", ev.Note, ev.Note, ev.Velocity)
	case san_piano.NoteOff:
		what = fmt.Sprintf("%-4s %3d  off", ev.Note, ev.Note)
	case san_piano.ControlChange:
		what = control(ev.Controller, ev.Value)
	case san_piano.ProgramChange:
		what = fmt.Sprintf("program %d", ev.Value)
	case san_piano.PolyPressure:
		what = fmt.Sprintf("%-4s %3d  pressure %3d", ev.Note, ev.Note, ev.Value)
	case san_piano.ChannelPressure:
		what = fmt.Sprintf("pressure %d", ev.Value)
	case san_piano.PitchBend:
		what = fmt.Sprintf("pitch bend %+d", ev.Value)
	default:
		status := ev.Raw[0]
		if status == 0xF8 || status == 0xFE { // clock, active sensing
			return ""
		}
		if what = systemNames[status]; what == "" {
			what = "system message"
		}
	}
	line := fmt.Sprintf("%9.3fs  % -8X  %s", ev.Time.Seconds(), ev.Bytes(), what)
	if ev.Kind != san_piano.System && ev.Channel != 0 {
		line += fmt.Sprintf("  (channel %d)", ev.Channel+1)
	}
	return line
}

func control(controller uint8, value int) string {
	pedal := map[uint8]string{
		san_piano.SustainPedal:   "sustain pedal",
		san_piano.SostenutoPedal: "sostenuto pedal",
		san_piano.SoftPedal:      "soft pedal",
	}[controller]
	if pedal == "" {
		return fmt.Sprintf("control %d = %d", controller, value)
	}
	state := "up"
	if value >= 64 {
		state = "down"
	}
	return fmt.Sprintf("%s %s (%d)", pedal, state, value)
}
