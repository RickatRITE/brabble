//go:build windows

package run

import "os"

func stopSignals() []os.Signal {
	return []os.Signal{os.Interrupt}
}
