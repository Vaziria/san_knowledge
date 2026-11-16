package san_backtest

import (
	"os/exec"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// createNoWindow is CREATE_NO_WINDOW: a console program started from a
// windowless host (San Tower) would otherwise open a console window.
const createNoWindow = 0x08000000

// hideWindow starts the plugin without a console window.
func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
}

// killTree puts the started plugin in a job object that kills every
// process in it when the job closes (so also when the host dies), and
// returns the func that closes it. Without a job it kills the process.
func killTree(cmd *exec.Cmd) func() {
	var once sync.Once
	job, err := windows.CreateJobObject(nil, nil)
	if err == nil {
		info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
		info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
		_, err = windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)))
		if err == nil {
			var h windows.Handle
			h, err = windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(cmd.Process.Pid))
			if err == nil {
				err = windows.AssignProcessToJobObject(job, h)
				windows.CloseHandle(h)
			}
		}
		if err != nil {
			windows.CloseHandle(job)
		}
	}
	if err != nil {
		return func() { once.Do(func() { cmd.Process.Kill() }) }
	}
	return func() {
		once.Do(func() {
			windows.TerminateJobObject(job, 1)
			windows.CloseHandle(job)
		})
	}
}
