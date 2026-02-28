package ssh

import (
	"strings"
	"testing"
)

func TestCommandLine(t *testing.T) {
	args := &ConnectArgs{
		Username:   "mario.rossi",
		TargetUser: "root",
		Host:       "10.0.0.1",
		ProxyHost:  "psmp.sogei.it",
		KeyPath:    "/home/user/.sogark/keys/id_sogark",
	}

	cmd := args.CommandLine()
	if len(cmd) < 4 {
		t.Fatalf("command too short: %v", cmd)
	}
	if cmd[0] != "ssh" {
		t.Errorf("cmd[0]: got %q, want %q", cmd[0], "ssh")
	}
	expected := "mario.rossi@root@10.0.0.1@psmp.sogei.it"
	if cmd[1] != expected {
		t.Errorf("cmd[1]: got %q, want %q", cmd[1], expected)
	}
	if cmd[2] != "-i" {
		t.Errorf("cmd[2]: got %q, want %q", cmd[2], "-i")
	}
	if cmd[3] != "/home/user/.sogark/keys/id_sogark" {
		t.Errorf("cmd[3]: got %q, want %q", cmd[3], "/home/user/.sogark/keys/id_sogark")
	}
}

func TestCommandLine_WithExtraArgs(t *testing.T) {
	args := &ConnectArgs{
		Username:   "user",
		TargetUser: "root",
		Host:       "host",
		ProxyHost:  "proxy",
		KeyPath:    "/key",
		ExtraArgs:  []string{"-v", "-o", "StrictHostKeyChecking=no"},
	}

	cmd := args.CommandLine()
	if len(cmd) != 7 { // ssh, user@..., -i, /key, -v, -o, Strict...
		t.Errorf("expected 7 args, got %d: %v", len(cmd), cmd)
	}
	if cmd[4] != "-v" {
		t.Errorf("extra arg[0]: got %q, want %q", cmd[4], "-v")
	}
}

func TestCommandString(t *testing.T) {
	args := &ConnectArgs{
		Username:   "user",
		TargetUser: "root",
		Host:       "host",
		ProxyHost:  "proxy",
		KeyPath:    "/key",
	}

	s := args.CommandString()
	if !strings.HasPrefix(s, "ssh ") {
		t.Errorf("command string should start with 'ssh': %q", s)
	}
	if !strings.Contains(s, "user@root@host@proxy") {
		t.Errorf("command string missing user string: %q", s)
	}
	if !strings.Contains(s, "-i /key") {
		t.Errorf("command string missing key flag: %q", s)
	}
}

func TestParseTarget_WithUser(t *testing.T) {
	user, host := ParseTarget("admin@webserver", "root")
	if user != "admin" {
		t.Errorf("user: got %q, want %q", user, "admin")
	}
	if host != "webserver" {
		t.Errorf("host: got %q, want %q", host, "webserver")
	}
}

func TestParseTarget_WithoutUser(t *testing.T) {
	user, host := ParseTarget("webserver", "root")
	if user != "root" {
		t.Errorf("user: got %q, want %q", user, "root")
	}
	if host != "webserver" {
		t.Errorf("host: got %q, want %q", host, "webserver")
	}
}

func TestParseTarget_IPAddress(t *testing.T) {
	user, host := ParseTarget("10.0.0.1", "root")
	if user != "root" {
		t.Errorf("user: got %q, want %q", user, "root")
	}
	if host != "10.0.0.1" {
		t.Errorf("host: got %q, want %q", host, "10.0.0.1")
	}
}

func TestParseTarget_UserAtIP(t *testing.T) {
	user, host := ParseTarget("admin@10.0.0.1", "root")
	if user != "admin" {
		t.Errorf("user: got %q, want %q", user, "admin")
	}
	if host != "10.0.0.1" {
		t.Errorf("host: got %q, want %q", host, "10.0.0.1")
	}
}

func TestParseTarget_EmptyDefaultUser(t *testing.T) {
	user, host := ParseTarget("webserver", "")
	if user != "" {
		t.Errorf("user: got %q, want empty", user)
	}
	if host != "webserver" {
		t.Errorf("host: got %q, want %q", host, "webserver")
	}
}

func TestConnectArgs_UserFormat(t *testing.T) {
	// Verify the CyberArk PSMP user format: corporate@target@host@proxy
	args := &ConnectArgs{
		Username:   "m.rossi",
		TargetUser: "admin",
		Host:       "server1.internal",
		ProxyHost:  "psmp.corp.com",
		KeyPath:    "/key",
	}
	cmd := args.CommandLine()
	if cmd[1] != "m.rossi@admin@server1.internal@psmp.corp.com" {
		t.Errorf("unexpected user format: %q", cmd[1])
	}
}
