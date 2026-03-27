//go:build !darwin

package service

import "fmt"

// LaunchdParams contains values used to render a launchd plist.
type LaunchdParams struct {
	Label  string
	Binary string
	Config string
	Log    string
	Env    map[string]string
}

// LaunchdPath returns the plist path for a label (unsupported on this platform).
func LaunchdPath(label string) string {
	return ""
}

// WritePlist is not supported on non-macOS platforms.
func WritePlist(params LaunchdParams) (string, error) {
	return "", fmt.Errorf("launchd service management is only supported on macOS")
}

// Status is not supported on non-macOS platforms.
func Status(label string) (string, bool) {
	return "", false
}
