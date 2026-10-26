//go:build windows

package main

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// killChildrenOnExit puts dev in a job object that kills every process in
// it once its handle closes, which Windows does when dev ends, however it
// ends: Ctrl+C, a closed window, Task Manager. What dev starts joins the
// job: Vite, the tower, its builds, and what the tower starts (claude). A
// tower stops by itself when its stdin closes, but Vite shares the console
// and went on (2026-10-27).
func killChildrenOnExit() error {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return err
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE},
	}
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		windows.CloseHandle(job)
		return err
	}
	if err := windows.AssignProcessToJobObject(job, windows.CurrentProcess()); err != nil {
		windows.CloseHandle(job)
		return err
	}
	// The handle stays open as long as dev runs.
	return nil
}

// processAlive says whether the process pid is running.
func processAlive(pid int) bool {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(h)
	var code uint32
	if err := windows.GetExitCodeProcess(h, &code); err != nil {
		return false
	}
	const stillActive = 259
	return code == stillActive
}
