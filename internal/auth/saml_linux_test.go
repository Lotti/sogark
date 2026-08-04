//go:build linux

package auth

import (
	"errors"
	"testing"
)

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

func TestFindBrowserInPath_PrefersKnownCandidates(t *testing.T) {
	calls := []string{}
	path := findBrowserInPath(func(name string) (string, error) {
		calls = append(calls, name)
		if name == "chromium-browser" {
			return "/usr/bin/chromium-browser", nil
		}
		return "", errors.New("not found")
	})

	if path != "/usr/bin/chromium-browser" {
		t.Fatalf("path = %q, want chromium-browser path", path)
	}
	if len(calls) == 0 || calls[0] != "microsoft-edge" {
		t.Fatalf("unexpected candidate order: %v", calls)
	}
}

func TestFindBrowserInPath_ReturnsEmptyWhenMissing(t *testing.T) {
	path := findBrowserInPath(func(name string) (string, error) {
		return "", errors.New("not found")
	})
	if path != "" {
		t.Fatalf("path = %q, want empty", path)
	}
}
