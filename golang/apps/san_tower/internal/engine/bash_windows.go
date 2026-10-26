//go:build windows

package engine

import (
	"os/exec"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// runTree runs cmd in a job object of its own, so a timeout or a stop ends
// the script and everything it started: msys's fork loses the parent links
// that taskkill /T follows (2026-10-27: a `sleep 30 &` outlived it). It
// starts suspended, joins the job, then runs, so nothing it starts gets out
// first. What it leaves running when it ends by itself goes on, as in a
// terminal.
func runTree(cmd *exec.Cmd) error {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		cmd.Cancel = func() error { return cmd.Process.Kill() }
		return cmd.Run()
	}
	defer windows.CloseHandle(job)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_SUSPENDED}
	cmd.Cancel = func() error { return windows.TerminateJobObject(job, 1) }
	if err := cmd.Start(); err != nil {
		return err
	}
	if err := joinJob(job, cmd.Process.Pid); err != nil {
		cmd.Process.Kill()
		cmd.Wait()
		return err
	}
	if err := resumeProcess(cmd.Process.Pid); err != nil {
		windows.TerminateJobObject(job, 1)
		cmd.Wait()
		return err
	}
	return cmd.Wait()
}

func joinJob(job windows.Handle, pid int) error {
	h, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	return windows.AssignProcessToJobObject(job, h)
}

// resumeProcess resumes the threads of process pid, which started
// suspended.
func resumeProcess(pid int) error {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(snap)
	te := windows.ThreadEntry32{Size: uint32(unsafe.Sizeof(windows.ThreadEntry32{}))}
	for err = windows.Thread32First(snap, &te); err == nil; err = windows.Thread32Next(snap, &te) {
		if te.OwnerProcessID != uint32(pid) {
			continue
		}
		th, err := windows.OpenThread(windows.THREAD_SUSPEND_RESUME, false, te.ThreadID)
		if err != nil {
			return err
		}
		_, err = windows.ResumeThread(th)
		windows.CloseHandle(th)
		if err != nil {
			return err
		}
	}
	return nil
}
