//go:build linux

package auth

import (
	"errors"
	"testing"
)

func TestEnsureLinuxAuthMode_DefaultsToGUIWithDisplay(t *testing.T) {
	t.Setenv(envAuthMode, "")
	t.Setenv("DISPLAY", ":0")
	t.Setenv("WAYLAND_DISPLAY", "")

	if err := ensureLinuxAuthMode(); err != nil {
		t.Fatalf("ensureLinuxAuthMode() error: %v", err)
	}
}

func TestEnsureLinuxAuthMode_RequiresGUIByDefault(t *testing.T) {
	t.Setenv(envAuthMode, "")
	t.Setenv("DISPLAY", "")
	t.Setenv("WAYLAND_DISPLAY", "")

	if err := ensureLinuxAuthMode(); err == nil {
		t.Fatal("ensureLinuxAuthMode() should fail without GUI")
	}
}

func TestEnsureLinuxAuthMode_RejectsHeadless(t *testing.T) {
	t.Setenv(envAuthMode, "headless")
	t.Setenv("DISPLAY", ":0")

	if err := ensureLinuxAuthMode(); err == nil {
		t.Fatal("ensureLinuxAuthMode() should reject headless mode")
	}
}

func TestEnsureLinuxAuthMode_Invalid(t *testing.T) {
	t.Setenv(envAuthMode, "tty-browser")

	if err := ensureLinuxAuthMode(); err == nil {
		t.Fatal("ensureLinuxAuthMode() should fail")
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
