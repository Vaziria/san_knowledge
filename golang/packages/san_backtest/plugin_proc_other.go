//go:build !windows

package san_backtest

import (
	"os/exec"
	"sync"
	"syscall"
)

func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killTree kills the plugin's process group (the plugin and anything it
// started).
func killTree(cmd *exec.Cmd) func() {
	var once sync.Once
	return func() {
		once.Do(func() {
			syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			cmd.Process.Kill()
		})
	}
}
