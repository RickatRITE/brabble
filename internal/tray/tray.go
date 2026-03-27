// Package tray provides a Windows system-tray UI for the brabble daemon.
package tray

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"time"

	"brabble/internal/config"
	"brabble/internal/control"

	"fyne.io/systray"
)

// Run launches the system tray icon and blocks until quit.
func Run(cfg *config.Config) {
	systray.Run(func() { onReady(cfg) }, func() {})
}

func onReady(cfg *config.Config) {
	systray.SetIcon(iconData())
	systray.SetTitle("Brabble")
	systray.SetTooltip("Brabble — voice hook daemon")

	// ── Status header (disabled, informational) ──
	mStatus := systray.AddMenuItem("Status: checking…", "Daemon status")
	mStatus.Disable()

	systray.AddSeparator()

	// ── Daemon controls ──
	mStart := systray.AddMenuItem("Start Daemon", "Start the brabble daemon")
	mStop := systray.AddMenuItem("Stop Daemon", "Stop the brabble daemon")
	mRestart := systray.AddMenuItem("Restart Daemon", "Restart the brabble daemon")

	systray.AddSeparator()

	// ── Transcripts submenu ──
	mTranscripts := systray.AddMenuItem("Recent Transcripts", "Last heard utterances")
	mTranscripts.Disable()
	transcriptItems := make([]*systray.MenuItem, 5)
	for i := range transcriptItems {
		transcriptItems[i] = mTranscripts.AddSubMenuItem("—", "")
		transcriptItems[i].Disable()
	}

	systray.AddSeparator()

	// ── Utilities ──
	mOpenConfig := systray.AddMenuItem("Open Config…", "Open config file in editor")
	mOpenLog := systray.AddMenuItem("Open Log…", "Open log file in editor")
	mDoctor := systray.AddMenuItem("Run Doctor", "Check dependencies")

	systray.AddSeparator()

	// ── Help ──
	mHelp := systray.AddMenuItem("Help / Quick Reference", "Show keyboard shortcuts and usage")

	systray.AddSeparator()

	mQuit := systray.AddMenuItem("Quit Tray", "Close tray icon (daemon keeps running)")

	// ── Background status poller ──
	go pollStatus(cfg, mStatus, transcriptItems)

	// ── Event loop ──
	for {
		select {
		case <-mStart.ClickedCh:
			go runBrabble("start")
		case <-mStop.ClickedCh:
			go runBrabble("stop")
		case <-mRestart.ClickedCh:
			go runBrabble("restart")
		case <-mOpenConfig.ClickedCh:
			go openFile(cfg.Paths.ConfigPath)
		case <-mOpenLog.ClickedCh:
			go openFile(cfg.Paths.LogPath)
		case <-mDoctor.ClickedCh:
			go runBrabbleVisible("doctor")
		case <-mHelp.ClickedCh:
			go showHelp()
		case <-mQuit.ClickedCh:
			systray.Quit()
			return
		}
	}
}

// pollStatus periodically queries the daemon and updates tray state.
func pollStatus(cfg *config.Config, mStatus *systray.MenuItem, transcriptItems []*systray.MenuItem) {
	for {
		status, err := queryStatus(cfg)
		if err != nil {
			mStatus.SetTitle("Status: stopped")
			mStatus.SetTooltip(err.Error())
			systray.SetTooltip("Brabble — stopped")
			for _, item := range transcriptItems {
				item.SetTitle("—")
			}
		} else {
			uptime := formatUptime(status.UptimeSec)
			title := fmt.Sprintf("Status: running (%s)", uptime)
			mStatus.SetTitle(title)
			mStatus.SetTooltip("Daemon is running")
			systray.SetTooltip(fmt.Sprintf("Brabble — running %s", uptime))

			// Fill transcript slots (most recent first)
			for i := range transcriptItems {
				idx := len(status.Transcripts) - 1 - i
				if idx >= 0 {
					t := status.Transcripts[idx]
					line := fmt.Sprintf("%s  %s", t.Timestamp.Format("15:04:05"), truncate(t.Text, 60))
					transcriptItems[i].SetTitle(line)
				} else {
					transcriptItems[i].SetTitle("—")
				}
			}
		}
		time.Sleep(3 * time.Second)
	}
}

func queryStatus(cfg *config.Config) (*control.Status, error) {
	conn, err := net.DialTimeout("unix", cfg.Paths.SocketPath, 2*time.Second)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	req := control.Request{Op: "status"}
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return nil, err
	}
	var status control.Status
	if err := json.NewDecoder(conn).Decode(&status); err != nil {
		return nil, err
	}
	return &status, nil
}

func runBrabble(args ...string) {
	self, err := os.Executable()
	if err != nil {
		return
	}
	cmd := exec.Command(self, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
}

func runBrabbleVisible(args ...string) {
	self, err := os.Executable()
	if err != nil {
		return
	}
	// On Windows, open a visible console window so the user can see output.
	if runtime.GOOS == "windows" {
		cmdArgs := append([]string{"/C", self}, args...)
		cmdArgs = append(cmdArgs, "& pause")
		cmd := exec.Command("cmd", cmdArgs...)
		_ = cmd.Start()
		return
	}
	cmd := exec.Command(self, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
}

func openFile(path string) {
	if path == "" {
		return
	}
	switch runtime.GOOS {
	case "windows":
		_ = exec.Command("cmd", "/C", "start", "", path).Start()
	case "darwin":
		_ = exec.Command("open", path).Start()
	default:
		_ = exec.Command("xdg-open", path).Start()
	}
}

func showHelp() {
	help := `Brabble — Quick Reference
═══════════════════════════════════════

  TRAY ICON
  ─────────
  Right-click          Open this menu
  Start / Stop / Restart   Control the daemon
  Recent Transcripts   Last 5 things heard
  Open Config…         Edit ~/.config/brabble/config.toml
  Open Log…            View daemon log
  Run Doctor           Check deps & config
  Quit Tray            Close tray (daemon keeps running)

  CLI COMMANDS
  ────────────
  brabble start [--no-wake] [--metrics-addr host:port]
  brabble stop
  brabble restart
  brabble status [--json]
  brabble health
  brabble mic list [--json]
  brabble mic set <name> | --index N
  brabble models list | download <m> | set <m>
  brabble setup               Download default model
  brabble doctor              Check dependencies
  brabble test-hook "text"    Test hook manually
  brabble transcribe file.wav Transcribe a WAV file
  brabble tail-log            Show last 50 log lines
  brabble tray                Launch this tray icon

  WAKE WORD
  ─────────
  Say "clawd" (or "claude") then your command.
  Use --no-wake to skip wake word detection.

  CONFIG
  ──────
  Config: %APPDATA%\brabble\config.toml  (Windows)
          ~/.config/brabble/config.toml   (macOS/Linux)

  Key settings:
    [audio]  device_name, sample_rate
    [vad]    silence_ms, aggressiveness
    [asr]    model_path, language
    [wake]   enabled, word, aliases
    [hook]   command, args, prefix, cooldown_sec

  ENVIRONMENT VARIABLES
  ─────────────────────
  BRABBLE_WAKE_ENABLED=0         Disable wake word
  BRABBLE_METRICS_ADDR=host:port Enable Prometheus metrics
  BRABBLE_LOG_LEVEL=debug        Set log level
  BRABBLE_LOG_FORMAT=json        Set log format
  BRABBLE_TRANSCRIPTS_ENABLED=0  Disable transcript log
  BRABBLE_REDACT_PII=1           Redact emails/phones
`

	if runtime.GOOS == "windows" {
		// Write to a temp file and open it so it's readable
		tmp := os.TempDir() + "/brabble-help.txt"
		_ = os.WriteFile(tmp, []byte(help), 0o644)
		_ = exec.Command("cmd", "/C", "start", "", tmp).Start()
	} else {
		fmt.Print(help)
	}
}

func formatUptime(sec float64) string {
	d := time.Duration(sec * float64(time.Second))
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}
