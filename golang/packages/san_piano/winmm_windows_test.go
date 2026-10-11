//go:build windows

package san_piano

import "testing"

// TestDevices calls WinMM for real. It needs no instrument: with none
// plugged in the list is just empty.
func TestDevices(t *testing.T) {
	list, err := Devices()
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range list {
		t.Logf("%d  %s", d.ID, d.Name)
	}
}

func TestOpenUnknownID(t *testing.T) {
	if _, err := Open(-1); err == nil {
		t.Fatal("Open(-1) succeeded")
	}
}
