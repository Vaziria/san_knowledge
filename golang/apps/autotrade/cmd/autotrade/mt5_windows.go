package main

import (
	"os/exec"
	"syscall"
)

// compileCmd is MetaEditor's command line, written out by hand: it wants
// /compile:"path", which Go's own quoting of a path with spaces would not give.
func compileCmd(editor, src string) *exec.Cmd {
	cmd := exec.Command(editor)
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: `"` + editor + `" /compile:"` + src + `" /log`}
	return cmd
}
