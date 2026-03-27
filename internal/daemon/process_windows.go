//go:build windows

package daemon

import "syscall"

const (
	processTerminate            = 0x0001
	processQueryLimitedInfo     = 0x1000
	stillActive            uint32 = 259
)

// sendStopSignal terminates the process with the given PID on Windows.
// Windows doesn't have SIGTERM; we terminate the process directly.
func sendStopSignal(pid int) error {
	handle, err := syscall.OpenProcess(processTerminate, false, uint32(pid))
	if err != nil {
		return err
	}
	defer syscall.CloseHandle(handle)
	return syscall.TerminateProcess(handle, 1)
}

// isProcessAlive checks whether the process with the given PID is running on Windows.
func isProcessAlive(pid int) bool {
	handle, err := syscall.OpenProcess(processQueryLimitedInfo, false, uint32(pid))
	if err != nil {
		return false
	}
	defer syscall.CloseHandle(handle)
	var exitCode uint32
	if err := syscall.GetExitCodeProcess(handle, &exitCode); err != nil {
		return false
	}
	return exitCode == stillActive
}
