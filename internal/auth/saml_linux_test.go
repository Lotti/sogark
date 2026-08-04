//go:build linux

package auth

import "testing"

func TestResolveLinuxAuthMode_DefaultsToHeadlessWithoutDisplay(t *testing.T) {
	t.Setenv(envAuthMode, "")
	t.Setenv("DISPLAY", "")
	t.Setenv("WAYLAND_DISPLAY", "")

	mode, err := resolveLinuxAuthMode()
	if err != nil {
		t.Fatalf("resolveLinuxAuthMode() error: %v", err)
	}
	if mode != linuxAuthModeHeadless {
		t.Fatalf("mode = %q, want %q", mode, linuxAuthModeHeadless)
	}
}

func TestResolveLinuxAuthMode_DefaultsToGUIWithDisplay(t *testing.T) {
	t.Setenv(envAuthMode, "")
	t.Setenv("DISPLAY", ":0")
	t.Setenv("WAYLAND_DISPLAY", "")

	mode, err := resolveLinuxAuthMode()
	if err != nil {
		t.Fatalf("resolveLinuxAuthMode() error: %v", err)
	}
	if mode != linuxAuthModeGUI {
		t.Fatalf("mode = %q, want %q", mode, linuxAuthModeGUI)
	}
}

func TestResolveLinuxAuthMode_ExplicitOverride(t *testing.T) {
	t.Setenv(envAuthMode, "headless")
	t.Setenv("DISPLAY", ":0")

	mode, err := resolveLinuxAuthMode()
	if err != nil {
		t.Fatalf("resolveLinuxAuthMode() error: %v", err)
	}
	if mode != linuxAuthModeHeadless {
		t.Fatalf("mode = %q, want %q", mode, linuxAuthModeHeadless)
	}
}

func TestResolveLinuxAuthMode_Invalid(t *testing.T) {
	t.Setenv(envAuthMode, "tty-browser")

	if _, err := resolveLinuxAuthMode(); err == nil {
		t.Fatal("resolveLinuxAuthMode() should fail")
	}
}
