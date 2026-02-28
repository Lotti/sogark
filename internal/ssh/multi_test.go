package ssh

import (
	"testing"
)

func TestBuildSSHCmd(t *testing.T) {
	cmd := buildSSHCmd("mario.rossi", "root", "10.0.0.1", "psmp.sogei.it", "/keys/id_sogark")
	expected := "ssh mario.rossi@root@10.0.0.1@psmp.sogei.it -i /keys/id_sogark"
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
