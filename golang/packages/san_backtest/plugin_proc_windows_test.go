package san_backtest

import (
	"os/exec"
	"testing"
)

// A plugin started from San Tower (a program without a console) must not
// open a console window.
func TestPluginStartsHidden(t *testing.T) {
	cmd := exec.Command("plugin.exe")
	hideWindow(cmd)
	if a := cmd.SysProcAttr; a == nil || !a.HideWindow || a.CreationFlags&createNoWindow == 0 {
		t.Fatalf("SysProcAttr %+v", cmd.SysProcAttr)
	}
}
