//go:build !windows

package main

import "os/exec"

// compileCmd is nil off Windows: MetaEditor runs under Wine there, by hand.
func compileCmd(editor, src string) *exec.Cmd { return nil }
