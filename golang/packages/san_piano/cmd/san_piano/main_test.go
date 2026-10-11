package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	san_piano "github.com/wargasipil/san_piano"
)

func TestUsage(t *testing.T) {
	for _, args := range [][]string{nil, {"help"}, {"-h"}} {
		var stdout, stderr bytes.Buffer
		if err := run(context.Background(), args, &stdout, &stderr); err != nil {
			t.Errorf("run(%q): %v", args, err)
		}
		if !strings.Contains(stderr.String(), "san_piano debug") {
			t.Errorf("run(%q) printed no usage: %q", args, stderr.String())
		}
	}
}

func TestCommandErrors(t *testing.T) {
	for _, args := range [][]string{
		{"device"},
		{"device", "lst"},
		{"device", "list", "extra"},
		{"devices"},
		{"-device", "1"},
		{"debug", "extra"},
	} {
		var stdout, stderr bytes.Buffer
		err := run(context.Background(), args, &stdout, &stderr)
		if err == nil {
			t.Errorf("run(%q) succeeded", args)
		}
		if !strings.Contains(stderr.String(), "usage:") {
			t.Errorf("run(%q) printed no usage: %q", args, stderr.String())
		}
	}
}

// TestDebugBadFile fails on the missing folder when a piano is plugged in, and
// on the missing piano otherwise; either way it must not succeed.
func TestDebugBadFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "debug.txt")
	var stdout, stderr bytes.Buffer
	if err := run(context.Background(), []string{"debug", "-f", path}, &stdout, &stderr); err == nil {
		t.Fatal("debug -f into a missing folder succeeded")
	}
}

// TestDebugNoPianoKeepsFile: a run that finds no piano must not empty an
// earlier file. No input is named "no such piano", plugged in or not.
func TestDebugNoPianoKeepsFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "debug.txt")
	if err := os.WriteFile(path, []byte("earlier session\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	err := run(context.Background(), []string{"debug", "-device", "no such piano", "-f", path}, &stdout, &stderr)
	if !errors.Is(err, san_piano.ErrNoDevice) && !errors.Is(err, san_piano.ErrUnsupported) {
		t.Fatalf("err = %v, want ErrNoDevice", err)
	}
	if saved, _ := os.ReadFile(path); string(saved) != "earlier session\n" {
		t.Errorf("file now holds %q", saved)
	}
}

// TestShowToFile is what debug -f does once the input is open: the lines go
// to the screen and to the file alike.
func TestShowToFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "debug.txt")
	if err := os.WriteFile(path, []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	events := make(chan san_piano.Event, 3)
	events <- san_piano.Parse(0x90, 60, 87)
	events <- san_piano.Parse(0xFE, 0, 0) // active sensing: not shown
	events <- san_piano.Parse(0x80, 60, 64)
	close(events)

	var screen bytes.Buffer
	if err := show(events, io.MultiWriter(&screen, f)); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "    0.000s  90 3C 57  C4    60  on   velocity  87\n" +
		"    0.000s  80 3C 40  C4    60  off\n"
	if screen.String() != want || string(saved) != want {
		t.Errorf("screen %q\nfile   %q\nwant   %q", screen.String(), saved, want)
	}
}

// TestDeviceList lists the real MIDI inputs, so it needs Windows but no
// instrument: with none plugged in it says so on stderr.
func TestDeviceList(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("MIDI input is Windows only")
	}
	var stdout, stderr bytes.Buffer
	if err := run(context.Background(), []string{"device", "list"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if stdout.Len() == 0 && !strings.Contains(stderr.String(), "no MIDI inputs") {
		t.Errorf("no inputs listed and no message: stderr %q", stderr.String())
	}
	t.Logf("%s", stdout.String())
}

func TestFormat(t *testing.T) {
	at := func(ev san_piano.Event) san_piano.Event {
		ev.Time = 1512 * time.Millisecond
		return ev
	}
	tests := []struct {
		ev   san_piano.Event
		want string
	}{
		{at(san_piano.Parse(0x90, 60, 87)), "    1.512s  90 3C 57  C4    60  on   velocity  87"},
		{at(san_piano.Parse(0x90, 61, 0)), "    1.512s  90 3D 00  C#4   61  off"},
		{at(san_piano.Parse(0x80, 108, 64)), "    1.512s  80 6C 40  C8   108  off"},
		{at(san_piano.Parse(0xB0, 64, 127)), "    1.512s  B0 40 7F  sustain pedal down (127)"},
		{at(san_piano.Parse(0xB0, 67, 0)), "    1.512s  B0 43 00  soft pedal up (0)"},
		{at(san_piano.Parse(0xB0, 7, 100)), "    1.512s  B0 07 64  control 7 = 100"},
		{at(san_piano.Parse(0xC0, 5, 0)), "    1.512s  C0 05     program 5"},
		{at(san_piano.Parse(0xE0, 0, 0)), "    1.512s  E0 00 00  pitch bend -8192"},
		{at(san_piano.Parse(0x91, 60, 87)), "    1.512s  91 3C 57  C4    60  on   velocity  87  (channel 2)"},
		{at(san_piano.Parse(0xFA, 0, 0)), "    1.512s  FA        start"},
		{at(san_piano.Parse(0xFC, 0, 0)), "    1.512s  FC        stop"},
		{at(san_piano.Parse(0xF8, 0, 0)), ""},
		{at(san_piano.Parse(0xFE, 0, 0)), ""},
	}
	for _, tt := range tests {
		if got := format(tt.ev); got != tt.want {
			t.Errorf("format(%v)\n got %q\nwant %q", tt.ev.Raw, got, tt.want)
		}
	}
}
