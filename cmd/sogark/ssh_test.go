package main

import (
	"testing"
	"time"

	"github.com/Lotti/sogark/internal/config"
)

func TestParseSSHFlags_Basic(t *testing.T) {
	user, keyFormat, forceLogin, dryRun, host, sshArgs, err := parseSSHFlags([]string{"10.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	if host != "10.0.0.1" {
		t.Errorf("host = %q, want %q", host, "10.0.0.1")
	}
	if user != "" {
		t.Errorf("user = %q, want empty", user)
	}
	if keyFormat != "openssh" {
		t.Errorf("keyFormat = %q, want %q", keyFormat, "openssh")
	}
	if forceLogin {
		t.Error("forceLogin should be false")
	}
	if dryRun {
		t.Error("dryRun should be false")
	}
	if len(sshArgs) != 0 {
		t.Errorf("sshArgs = %v, want empty", sshArgs)
	}
}

func TestParseSSHFlags_WithUser(t *testing.T) {
	user, _, _, _, host, _, err := parseSSHFlags([]string{"-u", "oper1", "10.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	if user != "oper1" {
		t.Errorf("user = %q, want %q", user, "oper1")
	}
	if host != "10.0.0.1" {
		t.Errorf("host = %q, want %q", host, "10.0.0.1")
	}
}

func TestParseSSHFlags_UserEquals(t *testing.T) {
	user, _, _, _, _, _, err := parseSSHFlags([]string{"--user=oper1", "10.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	if user != "oper1" {
		t.Errorf("user = %q, want %q", user, "oper1")
	}
}

func TestParseSSHFlags_Flags(t *testing.T) {
	_, _, forceLogin, dryRun, _, _, err := parseSSHFlags([]string{"--force-login", "--dry-run", "host1"})
	if err != nil {
		t.Fatal(err)
	}
	if !forceLogin {
		t.Error("forceLogin should be true")
	}
	if !dryRun {
		t.Error("dryRun should be true")
	}
}

func TestParseSSHFlags_KeyFormat(t *testing.T) {
	_, keyFormat, _, _, _, _, err := parseSSHFlags([]string{"--key-format", "pem", "host1"})
	if err != nil {
		t.Fatal(err)
	}
	if keyFormat != "pem" {
		t.Errorf("keyFormat = %q, want %q", keyFormat, "pem")
	}
}

func TestParseSSHFlags_KeyFormatEquals(t *testing.T) {
	_, keyFormat, _, _, _, _, err := parseSSHFlags([]string{"--key-format=ppk", "host1"})
	if err != nil {
		t.Fatal(err)
	}
	if keyFormat != "ppk" {
		t.Errorf("keyFormat = %q, want %q", keyFormat, "ppk")
	}
}

func TestParseSSHFlags_SSHPassthrough(t *testing.T) {
	_, _, _, _, host, sshArgs, err := parseSSHFlags([]string{"host1", "-L", "8080:localhost:80"})
	if err != nil {
		t.Fatal(err)
	}
	if host != "host1" {
		t.Errorf("host = %q, want %q", host, "host1")
	}
	if len(sshArgs) != 2 || sshArgs[0] != "-L" || sshArgs[1] != "8080:localhost:80" {
		t.Errorf("sshArgs = %v, want [-L 8080:localhost:80]", sshArgs)
	}
}

func TestParseSSHFlags_MissingUserValue(t *testing.T) {
	_, _, _, _, _, _, err := parseSSHFlags([]string{"-u"})
	if err == nil {
		t.Fatal("expected error for missing -u value")
	}
}

func TestParseSSHFlags_Help(t *testing.T) {
	_, _, _, _, _, _, err := parseSSHFlags([]string{"--help"})
	if err == nil || err.Error() != "help" {
		t.Errorf("expected help error, got %v", err)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{2*time.Hour + 30*time.Minute, "2h 30m"},
		{1 * time.Hour, "1h 0m"},
		{45 * time.Minute, "45m"},
		{0, "0m"},
		{3*time.Hour + 59*time.Minute, "3h 59m"},
	}
	for _, tt := range tests {
		got := formatDuration(tt.d)
		if got != tt.want {
			t.Errorf("formatDuration(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

func TestParseSSHFlags_ConfigOverride(t *testing.T) {
	previous := config.FileOverride
	t.Cleanup(func() { config.FileOverride = previous })
	_, _, _, _, host, args, err := parseSSHFlags([]string{"--config", "/tmp/private.yaml", "host"})
	if err != nil || host != "host" || len(args) != 0 || config.FileOverride != "/tmp/private.yaml" {
		t.Fatalf("config override not consumed: host=%q args=%v err=%v", host, args, err)
	}
	_, _, _, _, _, _, err = parseSSHFlags([]string{"--config"})
	if err == nil {
		t.Fatal("missing config path must fail")
	}
}
