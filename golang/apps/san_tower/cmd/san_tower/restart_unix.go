//go:build !windows

package main

import (
	"os"
	"syscall"
)

// reexec starts the tower again from exe in this same process, which keeps
// its id, so whatever watches it (a run.sh loop, systemd) sees no exit.
func reexec(exe string) error {
	return syscall.Exec(exe, os.Args, os.Environ())
}
