package main

import (
	"bufio"
	"path/filepath"
	"strings"
	"testing"

	sshpkg "github.com/sogei/cyberark-cli/internal/ssh"
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

func TestPrompt_WithDefault(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("\n"))
	got := prompt(reader, "Test", "default_val")
	if got != "default_val" {
		t.Errorf("prompt with empty input = %q, want %q", got, "default_val")
	}
}

func TestPrompt_WithInput(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("custom\n"))
	got := prompt(reader, "Test", "default_val")
	if got != "custom" {
		t.Errorf("prompt with input = %q, want %q", got, "custom")
	}
}

func TestPrompt_NoDefault(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("value\n"))
	got := prompt(reader, "Test", "")
	if got != "value" {
		t.Errorf("prompt with no default = %q, want %q", got, "value")
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
