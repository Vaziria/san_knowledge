//go:build !windows

package main

import (
	"errors"
	"syscall"
)

// killChildrenOnExit does nothing here: the tower stops when its stdin
// closes, and Vite gets the terminal's hangup with dev.
func killChildrenOnExit() error { return nil }

// processAlive says whether the process pid is running.
func processAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}
