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
	// Expected: scp -i /home/mario/.sogark/keys/id_sogark file.txt mario.rossi@root@10.1.2.3@psmp.sogei.it:/tmp/
	want := []string{
		"scp", "-i", "/home/mario/.sogark/keys/id_sogark",
		"file.txt",
		"mario.rossi@root@10.1.2.3@psmp.sogei.it:/tmp/",
	}
	if len(got) != len(want) {
		t.Fatalf("CommandLine() length = %d, want %d\ngot:  %v\nwant: %v", len(got), len(want), got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("CommandLine()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
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
	want := []string{
		"scp", "-i", "/keys/id_sogark",
		"-r", "./mydir",
		"mario.rossi@admin@10.1.2.3@psmp.sogei.it:/opt/",
	}
	if len(got) != len(want) {
		t.Fatalf("CommandLine() length = %d, want %d\ngot:  %v\nwant: %v", len(got), len(want), got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("CommandLine()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
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
	want := []string{
		"scp", "-i", "/keys/id_sogark",
		"mario.rossi@root@10.1.2.3@psmp.sogei.it:/etc/hosts",
		"./local/",
	}
	if len(got) != len(want) {
		t.Fatalf("CommandLine() length = %d, want %d\ngot:  %v\nwant: %v", len(got), len(want), got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("CommandLine()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
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
	want := []string{
		"scp", "-i", "/keys/id_sogark",
		"-C", "-v", "-P", "2222",
		"file.txt",
		"mario.rossi@root@host@psmp.sogei.it:/tmp/",
	}
	if len(got) != len(want) {
		t.Fatalf("CommandLine() length = %d, want %d\ngot:  %v\nwant: %v", len(got), len(want), got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("CommandLine()[%d] = %q, want %q", i, got[i], want[i])
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
