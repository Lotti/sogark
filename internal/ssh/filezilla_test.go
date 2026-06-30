package ssh

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestConfigureFileZillaUsesProxyHostSeparatelyFromUser(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test assumes unix-style home layout")
	}

	home := t.TempDir()
	oldHome := os.Getenv("HOME")
	t.Setenv("HOME", home)
	if oldHome != "" {
		t.Cleanup(func() {
			_ = os.Setenv("HOME", oldHome)
		})
	}

	keyPath := filepath.Join(home, "id_sogark")
	if err := os.WriteFile(keyPath, []byte("dummy"), 0600); err != nil {
		t.Fatalf("write key: %v", err)
	}

	_, err := ConfigureFileZilla([]HostTarget{
		{
			Name:       "web1",
			Address:    "10.1.2.3",
			TargetUser: "oper1",
		},
	}, "mario.rossi", "psmp.example.com", keyPath)
	if err != nil {
		t.Fatalf("ConfigureFileZilla returned error: %v", err)
	}

	sitePath := filepath.Join(home, ".config", "filezilla", "sitemanager.xml")
	data, err := os.ReadFile(sitePath)
	if err != nil {
		t.Fatalf("read sitemanager.xml: %v", err)
	}

	var fz filezillaXML
	if err := xml.Unmarshal(data, &fz); err != nil {
		t.Fatalf("unmarshal sitemanager.xml: %v", err)
	}

	if len(fz.Servers.Servers) != 1 {
		t.Fatalf("server count = %d, want 1", len(fz.Servers.Servers))
	}

	server := fz.Servers.Servers[0]
	if server.Host != "psmp.example.com" {
		t.Fatalf("host = %q, want %q", server.Host, "psmp.example.com")
	}

	if server.User != "mario.rossi@oper1@10.1.2.3" {
		t.Fatalf("user = %q, want %q", server.User, "mario.rossi@oper1@10.1.2.3")
	}
}
