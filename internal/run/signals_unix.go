//go:build !windows

package run

import (
	"os"
	"syscall"
)

func stopSignals() []os.Signal {
	return []os.Signal{syscall.SIGTERM, syscall.SIGINT}
}
