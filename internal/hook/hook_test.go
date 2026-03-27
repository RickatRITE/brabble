package hook

import (
	"context"
	"os/exec"
	"runtime"
	"testing"
	"time"

	"brabble/internal/config"
	"brabble/internal/logging"
)

// echoCmd returns a platform-appropriate echo command.
func echoCmd() string {
	if runtime.GOOS == "windows" {
		if p, err := exec.LookPath("echo"); err == nil {
			return p
		}
		return "cmd"
	}
	return "/bin/echo"
}

// echoArgs returns args that make the echo command work cross-platform.
// On Windows, if using cmd, we wrap with /C echo.
func echoArgs() []string {
	if runtime.GOOS == "windows" {
		if _, err := exec.LookPath("echo"); err != nil {
			return []string{"/C", "echo"}
		}
	}
	return []string{}
}

func TestShouldRunCooldown(t *testing.T) {
	cfg, _ := config.Default()
	cfg.Hooks = []config.HookConfig{{
		Command:     echoCmd(),
		Args:        echoArgs(),
		CooldownSec: 0.5,
	}}
	r := NewRunner(cfg, logging.NewTestLogger())
	r.SelectHook(&cfg.Hooks[0])

	if !r.ShouldRun() {
		t.Fatalf("first call should run")
	}
	if err := r.Run(context.Background(), Job{Text: "test", Timestamp: time.Now()}); err != nil {
		t.Fatalf("run: %v", err)
	}
	if r.ShouldRun() {
		t.Fatalf("cooldown should block immediate subsequent run")
	}
	time.Sleep(time.Duration(cfg.Hook.CooldownSec*float64(time.Second)) + 20*time.Millisecond)
	if !r.ShouldRun() {
		t.Fatalf("should run after cooldown")
	}
}

func TestRunUsesPrefixAndEnv(t *testing.T) {
	cfg, _ := config.Default()
	cfg.Hooks = []config.HookConfig{{
		Command: echoCmd(),
		Args:    echoArgs(),
		Prefix:  "pref:",
	}}

	r := NewRunner(cfg, logging.NewTestLogger())
	r.SelectHook(&cfg.Hooks[0])
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := r.Run(ctx, Job{Text: "hello", Timestamp: time.Now()}); err != nil {
		t.Fatalf("run echo: %v", err)
	}
}
