//go:build windows

package main

import (
	"os"
	"os/exec"
)

// reexec starts the tower again from exe as a new process, in the same
// console, and this one ends: Windows has no exec. The new one listens once
// this one has let go of the address.
func reexec(exe string) error {
	c := exec.Command(exe, os.Args[1:]...)
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	return c.Start()
}
