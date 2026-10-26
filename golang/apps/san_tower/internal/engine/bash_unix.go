//go:build !windows

package engine

import (
	"os/exec"
	"syscall"
)

// runTree runs cmd as a process group of its own, so a timeout or a stop
// ends the script and everything it started: the group is killed whole.
// What it leaves running when it ends by itself goes on, as in a terminal.
func runTree(cmd *exec.Cmd) error {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	return cmd.Run()
}
