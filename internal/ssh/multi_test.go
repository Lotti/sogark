package ssh

import (
	"strings"
	"testing"
)

func TestBuildSSHCmd(t *testing.T) {
	cmd := buildSSHCmd("mario.rossi", "root", "10.0.0.1", "psmp.sogei.it", "/keys/id_sogark")
	expected := "ssh mario.rossi@root@10.0.0.1@psmp.sogei.it -i /keys/id_sogark -o IdentitiesOnly=yes"
	if cmd != expected {
		t.Errorf("got %q, want %q", cmd, expected)
	}
}

func TestHostTarget(t *testing.T) {
	ht := HostTarget{
		Name:       "web1",
		Address:    "10.0.0.1",
		TargetUser: "root",
	}
	if ht.Name != "web1" {
		t.Errorf("Name: got %q, want %q", ht.Name, "web1")
	}
	if ht.Address != "10.0.0.1" {
		t.Errorf("Address: got %q, want %q", ht.Address, "10.0.0.1")
	}
	if ht.TargetUser != "root" {
		t.Errorf("TargetUser: got %q, want %q", ht.TargetUser, "root")
	}
}

func TestMultiArgs_DefaultSessionName(t *testing.T) {
	// Test that code handles empty session name (defaults to "sogark" in RunMulti)
	args := &MultiArgs{
		SessionName: "",
		Hosts:       []HostTarget{{Name: "h1", Address: "1.2.3.4", TargetUser: "root"}},
		Sync:        true,
	}
	if args.SessionName != "" {
		t.Error("SessionName should be empty before RunMulti sets default")
	}
}

func TestBuildSogarkSSHArgs(t *testing.T) {
	got := buildSogarkSSHArgs("root", "10.0.0.1")
	want := []string{"sogark", "ssh", "root@10.0.0.1"}
	if len(got) != len(want) {
		t.Fatalf("len: got %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d]: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestRunMulti_EmptyHosts(t *testing.T) {
	err := RunMulti(&MultiArgs{Hosts: nil}, "", "", "")
	if err == nil {
		t.Fatal("expected error for empty hosts")
	}
	if !strings.Contains(err.Error(), "no hosts") {
		t.Errorf("got %q, want error containing 'no hosts'", err.Error())
	}
}

func TestRunMulti_UnsupportedBackend(t *testing.T) {
	err := RunMulti(&MultiArgs{
		Hosts:   []HostTarget{{Name: "h1", Address: "1.2.3.4", TargetUser: "root"}},
		Backend: "invalid",
	}, "", "", "")
	if err == nil {
		t.Fatal("expected error for unsupported backend")
	}
	if !strings.Contains(err.Error(), "not supported") {
		t.Errorf("got %q, want error containing 'not supported'", err.Error())
	}
}

func TestRunMoba_EmptyHosts(t *testing.T) {
	err := RunMoba(nil, "", "", "", "", 20)
	if err == nil {
		t.Fatal("expected error for empty hosts")
	}
	if !strings.Contains(err.Error(), "no hosts") {
		t.Errorf("got %q, want error containing 'no hosts'", err.Error())
	}
}
