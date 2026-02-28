package ssh

import (
	"runtime"
	"testing"
)

func TestParseRemotePath(t *testing.T) {
	tests := []struct {
		input    string
		wantHost string
		wantPath string
		wantOK   bool
	}{
		{"host:/path/to/file", "host", "/path/to/file", true},
		{"10.1.2.3:/tmp/data", "10.1.2.3", "/tmp/data", true},
		{"user@host:/path", "user@host", "/path", true},
		{"host:", "host", "", true},
		{"/local/path", "", "", false},
		{"./file.txt", "", "", false},
		{"file.txt", "", "", false},
		{"", "", "", false},
		{"-r", "", "", false},
	}

	for _, tt := range tests {
		host, path, ok := ParseRemotePath(tt.input)
		if ok != tt.wantOK || host != tt.wantHost || path != tt.wantPath {
			t.Errorf("ParseRemotePath(%q) = (%q, %q, %v), want (%q, %q, %v)",
				tt.input, host, path, ok, tt.wantHost, tt.wantPath, tt.wantOK)
		}
	}

	// Windows drive letter test
	if runtime.GOOS == "windows" {
		host, path, ok := ParseRemotePath("C:\\Users\\test")
		if ok {
			t.Errorf("ParseRemotePath(C:\\Users\\test) should return false on Windows, got (%q, %q, true)", host, path)
		}
	}
}

func TestScpArgs_CommandLine(t *testing.T) {
	args := &ScpArgs{
		Username:   "mario.rossi",
		TargetUser: "root",
		ProxyHost:  "psmp.sogei.it",
		KeyPath:    "/home/mario/.sogark/keys/id_sogark",
		ScpArgs:    []string{"file.txt", "10.1.2.3:/tmp/"},
	}

	got := args.CommandLine()

	// Base expected args (without -O which depends on local OpenSSH version)
	wantPrefix := []string{"scp", "-i", "/home/mario/.sogark/keys/id_sogark", "-o", "IdentitiesOnly=yes"}
	wantSuffix := []string{"file.txt", "mario.rossi@root@10.1.2.3@psmp.sogei.it:/tmp/"}

	assertCommandLine(t, got, wantPrefix, wantSuffix)
}

func TestScpArgs_CommandLine_UserOverride(t *testing.T) {
	args := &ScpArgs{
		Username:   "mario.rossi",
		TargetUser: "root",
		ProxyHost:  "psmp.sogei.it",
		KeyPath:    "/keys/id_sogark",
		ScpArgs:    []string{"-r", "./mydir", "admin@10.1.2.3:/opt/"},
	}

	got := args.CommandLine()
	wantPrefix := []string{"scp", "-i", "/keys/id_sogark", "-o", "IdentitiesOnly=yes"}
	wantSuffix := []string{"-r", "./mydir", "mario.rossi@admin@10.1.2.3@psmp.sogei.it:/opt/"}

	assertCommandLine(t, got, wantPrefix, wantSuffix)
}

func TestScpArgs_CommandLine_Download(t *testing.T) {
	args := &ScpArgs{
		Username:   "mario.rossi",
		TargetUser: "root",
		ProxyHost:  "psmp.sogei.it",
		KeyPath:    "/keys/id_sogark",
		ScpArgs:    []string{"10.1.2.3:/etc/hosts", "./local/"},
	}

	got := args.CommandLine()
	wantPrefix := []string{"scp", "-i", "/keys/id_sogark", "-o", "IdentitiesOnly=yes"}
	wantSuffix := []string{"mario.rossi@root@10.1.2.3@psmp.sogei.it:/etc/hosts", "./local/"}

	assertCommandLine(t, got, wantPrefix, wantSuffix)
}

func TestScpArgs_CommandLine_WithFlags(t *testing.T) {
	args := &ScpArgs{
		Username:   "mario.rossi",
		TargetUser: "root",
		ProxyHost:  "psmp.sogei.it",
		KeyPath:    "/keys/id_sogark",
		ScpArgs:    []string{"-C", "-v", "-P", "2222", "file.txt", "host:/tmp/"},
	}

	got := args.CommandLine()
	wantPrefix := []string{"scp", "-i", "/keys/id_sogark", "-o", "IdentitiesOnly=yes"}
	wantSuffix := []string{"-C", "-v", "-P", "2222", "file.txt", "mario.rossi@root@host@psmp.sogei.it:/tmp/"}

	assertCommandLine(t, got, wantPrefix, wantSuffix)
}

// assertCommandLine checks that got starts with wantPrefix, ends with wantSuffix,
// and optionally has "-O" in between (depends on local OpenSSH version).
func assertCommandLine(t *testing.T, got, wantPrefix, wantSuffix []string) {
	t.Helper()
	minLen := len(wantPrefix) + len(wantSuffix)
	maxLen := minLen + 1 // optional -O

	if len(got) < minLen || len(got) > maxLen {
		t.Fatalf("CommandLine() length = %d, want %d or %d\ngot: %v", len(got), minLen, maxLen, got)
	}

	for i, w := range wantPrefix {
		if got[i] != w {
			t.Errorf("CommandLine()[%d] = %q, want %q", i, got[i], w)
		}
	}

	suffixStart := len(got) - len(wantSuffix)
	for i, w := range wantSuffix {
		if got[suffixStart+i] != w {
			t.Errorf("CommandLine()[%d] = %q, want %q", suffixStart+i, got[suffixStart+i], w)
		}
	}

	// If there's an extra element, it must be -O
	if len(got) == maxLen {
		oIdx := len(wantPrefix)
		if got[oIdx] != "-O" {
			t.Errorf("CommandLine()[%d] = %q, want %q", oIdx, got[oIdx], "-O")
		}
	}
}

func TestHasRemoteArg(t *testing.T) {
	tests := []struct {
		args []string
		want bool
	}{
		{[]string{"file.txt", "host:/path"}, true},
		{[]string{"-r", "host:/path"}, true},
		{[]string{"file.txt", "./local"}, false},
		{[]string{"-C", "-v"}, false},
		{[]string{}, false},
	}

	for _, tt := range tests {
		got := HasRemoteArg(tt.args)
		if got != tt.want {
			t.Errorf("HasRemoteArg(%v) = %v, want %v", tt.args, got, tt.want)
		}
	}
}

func TestExpandBatchRemote(t *testing.T) {
	h := HostTarget{Name: "web1", Address: "10.0.0.1", TargetUser: "root"}

	tests := []struct {
		args []string
		want []string
	}{
		{[]string{"file.txt", ":/tmp/"}, []string{"file.txt", "10.0.0.1:/tmp/"}},
		{[]string{"-r", "./dir", ":/opt/app/"}, []string{"-r", "./dir", "10.0.0.1:/opt/app/"}},
		{[]string{"file.txt", "./local"}, []string{"file.txt", "./local"}},
		{[]string{":/etc/hosts", "./"}, []string{"10.0.0.1:/etc/hosts", "./"}},
	}

	for _, tt := range tests {
		got := ExpandBatchRemote(tt.args, h)
		if len(got) != len(tt.want) {
			t.Fatalf("ExpandBatchRemote(%v) length = %d, want %d", tt.args, len(got), len(tt.want))
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("ExpandBatchRemote(%v)[%d] = %q, want %q", tt.args, i, got[i], tt.want[i])
			}
		}
	}
}
