package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	sshpkg "github.com/Lotti/sogark/internal/ssh"
)

func TestFormatHostNames(t *testing.T) {
	tests := []struct {
		targets []sshpkg.HostTarget
		want    string
	}{
		{nil, ""},
		{[]sshpkg.HostTarget{{Name: "web1"}}, "web1"},
		{[]sshpkg.HostTarget{{Name: "web1"}, {Name: "db1"}, {Name: "app1"}}, "web1, db1, app1"},
	}
	for _, tt := range tests {
		got := formatHostNames(tt.targets)
		if got != tt.want {
			t.Errorf("formatHostNames(%v) = %q, want %q", tt.targets, got, tt.want)
		}
	}
}

func TestSplitCSV(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"a,b,c", []string{"a", "b", "c"}},
		{" a , b , c ", []string{"a", "b", "c"}},
		{"a,,b", []string{"a", "b"}},
		{"", nil},
		{"  ,  ,  ", nil},
		{"single", []string{"single"}},
	}
	for _, tt := range tests {
		got := splitCSV(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("splitCSV(%q) = %v (len %d), want %v (len %d)", tt.input, got, len(got), tt.want, len(tt.want))
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("splitCSV(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

func TestFallbackPrompter_WithDefault(t *testing.T) {
	var output bytes.Buffer
	prompter := newFallbackPrompter(strings.NewReader("\n"), &output)

	got, err := prompter.Prompt("Test", "default_val")
	if err != nil {
		t.Fatalf("Prompt() error = %v", err)
	}
	if got != "default_val" {
		t.Errorf("Prompt() with empty input = %q, want %q", got, "default_val")
	}
	if output.String() != "Test [default_val]: " {
		t.Errorf("Prompt() wrote %q, want %q", output.String(), "Test [default_val]: ")
	}
}

func TestFallbackPrompter_WithInput(t *testing.T) {
	prompter := newFallbackPrompter(strings.NewReader("custom\n"), &bytes.Buffer{})

	got, err := prompter.Prompt("Test", "default_val")
	if err != nil {
		t.Fatalf("Prompt() error = %v", err)
	}
	if got != "custom" {
		t.Errorf("Prompt() with input = %q, want %q", got, "custom")
	}
}

func TestFallbackPrompter_NoDefault(t *testing.T) {
	var output bytes.Buffer
	prompter := newFallbackPrompter(strings.NewReader("value\n"), &output)

	got, err := prompter.Prompt("Test", "")
	if err != nil {
		t.Fatalf("Prompt() error = %v", err)
	}
	if got != "value" {
		t.Errorf("Prompt() with no default = %q, want %q", got, "value")
	}
	if output.String() != "Test: " {
		t.Errorf("Prompt() wrote %q, want %q", output.String(), "Test: ")
	}
}

func TestKeyFilePaths(t *testing.T) {
	openssh, ppk, pem := keyFilePaths("/keys", "id_sogark")
	wantOpenssh := filepath.Join("/keys", "id_sogark")
	wantPpk := filepath.Join("/keys", "id_sogark.ppk")
	wantPem := filepath.Join("/keys", "id_sogark.pem")

	if openssh != wantOpenssh {
		t.Errorf("openssh = %q, want %q", openssh, wantOpenssh)
	}
	if ppk != wantPpk {
		t.Errorf("ppk = %q, want %q", ppk, wantPpk)
	}
	if pem != wantPem {
		t.Errorf("pem = %q, want %q", pem, wantPem)
	}
}

func TestResolveFileZillaKeyPathPrefersOpenSSH(t *testing.T) {
	keyDir := t.TempDir()
	openssh, ppk, pem := keyFilePaths(keyDir, "id_sogark")

	for _, path := range []string{openssh, ppk, pem} {
		if err := os.WriteFile(path, []byte("test"), 0600); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	got := resolveFileZillaKeyPath(keyDir, "id_sogark")
	if got != openssh {
		t.Fatalf("resolveFileZillaKeyPath() = %q, want %q", got, openssh)
	}
}

func TestResolveFileZillaKeyPathFallsBackToPPK(t *testing.T) {
	keyDir := t.TempDir()
	_, ppk, _ := keyFilePaths(keyDir, "id_sogark")

	if err := os.WriteFile(ppk, []byte("test"), 0600); err != nil {
		t.Fatalf("write %s: %v", ppk, err)
	}

	got := resolveFileZillaKeyPath(keyDir, "id_sogark")
	if got != ppk {
		t.Fatalf("resolveFileZillaKeyPath() = %q, want %q", got, ppk)
	}
}

func TestWeztermLuaConfig(t *testing.T) {
	lua := weztermLuaConfig()

	checks := []string{
		`prefer_egl = true`,
		`wezterm.action.CopyTo('Clipboard')`,
		`wezterm.action.PasteFrom('Clipboard')`,
		`local wezterm = require 'wezterm'`,
	}
	for _, c := range checks {
		if !strings.Contains(lua, c) {
			t.Errorf("weztermLuaConfig() missing %q", c)
		}
	}
}
