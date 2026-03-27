// Package tray provides a Windows system-tray UI for the brabble daemon.
package tray

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"brabble/internal/config"
	"brabble/internal/control"
	"brabble/internal/doctor"

	"fyne.io/systray"
)

// Run launches the system tray icon and blocks until quit.
func Run(cfg *config.Config) {
	systray.Run(func() { onReady(cfg) }, func() {})
}

// doctorItem pairs a check result with its menu item and fix action.
type doctorItem struct {
	menuItem *systray.MenuItem
	result   doctor.Result
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

	// ── Doctor submenu (actionable checks) ──
	mDoctor := systray.AddMenuItem("Doctor: checking…", "Dependency and config checks")
	mDoctorRefresh := mDoctor.AddSubMenuItem("↻ Refresh checks", "Re-run all checks")
	mDoctor.AddSubMenuItem("", "")  // visual separator
	// Pre-allocate slots for check results
	const maxChecks = 8
	doctorItems := make([]doctorItem, maxChecks)
	for i := range doctorItems {
		doctorItems[i].menuItem = mDoctor.AddSubMenuItem("", "")
		doctorItems[i].menuItem.Hide()
	}

	// ── Utilities ──
	systray.AddSeparator()
	mOpenConfig := systray.AddMenuItem("Open Config…", "Open config file in editor")
	mOpenLog := systray.AddMenuItem("Open Log…", "Open log file in editor")

	systray.AddSeparator()

	// ── Help ──
	mHelp := systray.AddMenuItem("Help / Quick Reference", "Show keyboard shortcuts and usage")

	systray.AddSeparator()

	mQuit := systray.AddMenuItem("Quit Tray", "Close tray icon (daemon keeps running)")

	// ── Initial doctor check ──
	refreshDoctor(cfg, mDoctor, doctorItems[:])

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
		case <-mDoctorRefresh.ClickedCh:
			go refreshDoctor(cfg, mDoctor, doctorItems[:])
		case <-mHelp.ClickedCh:
			go showHelp()
		case <-mQuit.ClickedCh:
			systray.Quit()
			return

		// Doctor fix actions — clicking a failed check triggers its fix
		case <-doctorItems[0].menuItem.ClickedCh:
			go handleDoctorFix(cfg, &doctorItems[0], mDoctor, doctorItems[:])
		case <-doctorItems[1].menuItem.ClickedCh:
			go handleDoctorFix(cfg, &doctorItems[1], mDoctor, doctorItems[:])
		case <-doctorItems[2].menuItem.ClickedCh:
			go handleDoctorFix(cfg, &doctorItems[2], mDoctor, doctorItems[:])
		case <-doctorItems[3].menuItem.ClickedCh:
			go handleDoctorFix(cfg, &doctorItems[3], mDoctor, doctorItems[:])
		case <-doctorItems[4].menuItem.ClickedCh:
			go handleDoctorFix(cfg, &doctorItems[4], mDoctor, doctorItems[:])
		case <-doctorItems[5].menuItem.ClickedCh:
			go handleDoctorFix(cfg, &doctorItems[5], mDoctor, doctorItems[:])
		case <-doctorItems[6].menuItem.ClickedCh:
			go handleDoctorFix(cfg, &doctorItems[6], mDoctor, doctorItems[:])
		case <-doctorItems[7].menuItem.ClickedCh:
			go handleDoctorFix(cfg, &doctorItems[7], mDoctor, doctorItems[:])
		}
	}
}

// refreshDoctor runs checks and updates the doctor submenu.
func refreshDoctor(cfg *config.Config, mDoctor *systray.MenuItem, items []doctorItem) {
	results := doctor.Run(cfg)
	failures := 0
	for i := range items {
		if i < len(results) {
			r := results[i]
			items[i].result = r
			if r.Pass {
				items[i].menuItem.SetTitle(fmt.Sprintf("✓ %s — %s", r.Name, truncate(r.Detail, 50)))
				items[i].menuItem.SetTooltip("Check passed")
				items[i].menuItem.Disable()
			} else {
				failures++
				fix := fixLabel(r.Name)
				items[i].menuItem.SetTitle(fmt.Sprintf("✗ %s — %s  → %s", r.Name, truncate(r.Detail, 40), fix))
				items[i].menuItem.SetTooltip("Click to fix: " + fix)
				items[i].menuItem.Enable()
			}
			items[i].menuItem.Show()
		} else {
			items[i].menuItem.Hide()
		}
	}
	if failures == 0 {
		mDoctor.SetTitle("Doctor: all checks passed ✓")
	} else {
		mDoctor.SetTitle(fmt.Sprintf("Doctor: %d issue(s) found", failures))
	}
}

// fixLabel returns a short action description for a failed check.
func fixLabel(checkName string) string {
	switch checkName {
	case "model file":
		return "Download model"
	case "hook.command":
		return "Open config"
	case "config path":
		return "Create config"
	case "portaudio":
		return "Show install guide"
	case "pkg-config":
		return "Show install guide"
	default:
		return "Show details"
	}
}

// handleDoctorFix runs the appropriate fix for a failed check.
func handleDoctorFix(cfg *config.Config, item *doctorItem, mDoctor *systray.MenuItem, allItems []doctorItem) {
	if item.result.Pass {
		return
	}

	switch item.result.Name {
	case "model file":
		// Download the default whisper model
		item.menuItem.SetTitle("✗ model file — downloading…")
		item.menuItem.Disable()
		if err := downloadModel(cfg); err != nil {
			item.menuItem.SetTitle(fmt.Sprintf("✗ model file — download failed: %s  → Download model", truncate(err.Error(), 30)))
			item.menuItem.Enable()
			return
		}
		// Reload config and refresh
		newCfg, _ := config.Load(cfg.Paths.ConfigPath)
		if newCfg != nil {
			*cfg = *newCfg
		}
		refreshDoctor(cfg, mDoctor, allItems)

	case "hook.command":
		// Open config file so user can set hook.command
		openFile(cfg.Paths.ConfigPath)

	case "config path":
		// Create default config
		if err := config.Save(cfg, cfg.Paths.ConfigPath); err == nil {
			refreshDoctor(cfg, mDoctor, allItems)
		}

	case "portaudio", "pkg-config":
		showInstallGuide(item.result.Name)

	default:
		// Generic: show detail in a text file
		tmp := os.TempDir() + `\brabble-doctor-detail.txt`
		detail := fmt.Sprintf("Check: %s\r\nStatus: FAIL\r\nDetail: %s\r\n", item.result.Name, item.result.Detail)
		_ = os.WriteFile(tmp, []byte(detail), 0o644)
		openFile(tmp)
	}
}

// downloadModel downloads the default whisper model (same logic as setup command).
func downloadModel(cfg *config.Config) error {
	name := "ggml-large-v3-turbo-q8_0.bin"
	url := "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-large-v3-turbo-q8_0.bin"
	modelPath := filepath.Join(cfg.Paths.StateDir, "models", name)

	if err := os.MkdirAll(filepath.Dir(modelPath), 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(modelPath); err == nil {
		// Already present — just update config
		cfg.ASR.ModelPath = modelPath
		return config.Save(cfg, cfg.Paths.ConfigPath)
	}

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP %s", resp.Status)
	}

	tmp := modelPath + ".part"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, resp.Body); err != nil {
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, modelPath); err != nil {
		return err
	}

	cfg.ASR.ModelPath = modelPath
	return config.Save(cfg, cfg.Paths.ConfigPath)
}

func showInstallGuide(check string) {
	var guide string
	switch check {
	case "portaudio":
		switch runtime.GOOS {
		case "darwin":
			guide = "Install PortAudio:\r\n\r\n  brew install portaudio\r\n"
		case "windows":
			guide = "PortAudio for Windows:\r\n\r\n" +
				"If using MSYS2/MinGW:\r\n" +
				"  pacman -S mingw-w64-x86_64-portaudio\r\n\r\n" +
				"Ensure the PortAudio DLL is in your PATH or next to brabble.exe.\r\n"
		default:
			guide = "Install PortAudio:\r\n\r\n  sudo apt-get install libportaudio2 libportaudio-dev\r\n"
		}
	case "pkg-config":
		guide = "Install pkg-config:\r\n\r\n"
		if runtime.GOOS == "darwin" {
			guide += "  brew install pkg-config\r\n"
		} else {
			guide += "  sudo apt-get install pkg-config\r\n"
		}
	}
	tmp := os.TempDir() + `\brabble-install-guide.txt`
	_ = os.WriteFile(tmp, []byte(guide), 0o644)
	openFile(tmp)
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
  Doctor               Check deps — click failures to fix
  Open Config…         Edit config.toml
  Open Log…            View daemon log
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
		tmp := os.TempDir() + `\brabble-help.txt`
		_ = os.WriteFile(tmp, []byte(help), 0o644)
		openFile(tmp)
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
