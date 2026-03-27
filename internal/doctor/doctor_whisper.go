package doctor

import (
	"fmt"
	"runtime"

	"github.com/gordonklaus/portaudio"
)

func portaudioInstallHint() string {
	switch runtime.GOOS {
	case "darwin":
		return "install with: brew install portaudio"
	case "windows":
		return "ensure PortAudio DLL is in PATH or alongside the executable"
	default:
		return "install with: apt-get install libportaudio2 (or equivalent)"
	}
}

func checkPortAudio(_ bool) Result {
	if err := portaudio.Initialize(); err != nil {
		return Result{Name: "portaudio", Pass: false, Detail: fmt.Sprintf("init failed: %v (%s)", err, portaudioInstallHint())}
	}
	defer func() {
		_ = portaudio.Terminate()
	}()
	return Result{Name: "portaudio", Pass: true, Detail: "ok"}
}
