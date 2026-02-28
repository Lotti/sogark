package hosts

import (
	"strings"
	"testing"
)

func TestBuildSSHConfigEntry(t *testing.T) {
	entry := buildSSHConfigEntry("myhost", "10.0.0.1", "mario.rossi", "root", "psmp.sogei.it", "/home/user/.sogark/keys/id_sogark")

	mustContain := []string{
		"# --- sogark:myhost ---",
		"Host myhost",
		"HostName psmp.sogei.it",
		"User mario.rossi@root@10.0.0.1",
		"IdentityFile /home/user/.sogark/keys/id_sogark",
		"# --- /sogark:myhost ---",
	}

	for _, s := range mustContain {
		if !strings.Contains(entry, s) {
			t.Errorf("entry missing %q\nGot:\n%s", s, entry)
		}
	}
}

func TestRemoveSSHConfigEntry(t *testing.T) {
	content := `Host other
    HostName other.example.com

# --- sogark:myhost ---
Host myhost
    HostName psmp.sogei.it
    User user@root@10.0.0.1
    IdentityFile /path/to/key
# --- /sogark:myhost ---

Host another
    HostName another.example.com
`
	result := removeSSHConfigEntry(content, "myhost")

	if strings.Contains(result, "sogark:myhost") {
		t.Error("sogark entry should be removed")
	}
	if !strings.Contains(result, "Host other") {
		t.Error("other entries should remain")
	}
	if !strings.Contains(result, "Host another") {
		t.Error("another entry should remain")
	}
}

func TestRemoveSSHConfigEntry_NotFound(t *testing.T) {
	content := "Host other\n    HostName example.com\n"
	result := removeSSHConfigEntry(content, "nonexistent")
	if result != content {
		t.Error("content should be unchanged when entry not found")
	}
}

func TestRemoveSSHConfigEntry_Multiple(t *testing.T) {
	entry1 := "# --- sogark:host1 ---\nHost host1\n    HostName proxy\n# --- /sogark:host1 ---\n"
	entry2 := "# --- sogark:host2 ---\nHost host2\n    HostName proxy\n# --- /sogark:host2 ---\n"
	content := entry1 + entry2

	result := removeSSHConfigEntry(content, "host1")
	if strings.Contains(result, "sogark:host1") {
		t.Error("host1 entry should be removed")
	}
	if !strings.Contains(result, "sogark:host2") {
		t.Error("host2 entry should remain")
	}
}

func TestBuildSSHConfigEntry_ProxyIsHostName(t *testing.T) {
	// In CyberArk PSMP, HostName is the proxy, not the target
	entry := buildSSHConfigEntry("myhost", "target.server.com", "user", "root", "psmp.example.com", "/key")
	if !strings.Contains(entry, "HostName psmp.example.com") {
		t.Error("HostName should be the proxy host")
	}
	if !strings.Contains(entry, "User user@root@target.server.com") {
		t.Error("User should contain the full CyberArk user string")
	}
}
