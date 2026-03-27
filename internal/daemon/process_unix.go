//go:build !windows

package daemon

import (
	"os"
	"syscall"
)

// sendStopSignal sends SIGTERM to the process with the given PID.
func sendStopSignal(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Signal(syscall.SIGTERM)
}

// isProcessAlive checks whether the process with the given PID is running.
func isProcessAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}
