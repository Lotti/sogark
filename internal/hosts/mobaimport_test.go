package hosts

import (
	"testing"
)

func TestParseMobaContent_SSHSessions(t *testing.T) {
	content := `[Bookmarks]
SubRep=
ImgNum=42
Server1=#109#0%10.0.0.1%22%root%%-1%-1%%%%%0%0%0%%%-1%0%0%0%%1080%%0%0%1%
Server2=#109#0%10.0.0.2%22%admin%%-1%-1%%%%%0%0%0%%%-1%0%0%0%%1080%%0%0%1%
`
	sessions, err := ParseMobaContent(content)
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 2 {
		t.Fatalf("got %d sessions, want 2", len(sessions))
	}

	if sessions[0].Name != "Server1" || sessions[0].Address != "10.0.0.1" || sessions[0].User != "root" {
		t.Errorf("session 0: got %+v", sessions[0])
	}
	if sessions[1].Name != "Server2" || sessions[1].Address != "10.0.0.2" || sessions[1].User != "admin" {
		t.Errorf("session 1: got %+v", sessions[1])
	}
	// Root folder has no tags
	if len(sessions[0].Tags) != 0 {
		t.Errorf("session 0 tags: got %v, want empty", sessions[0].Tags)
	}
}

func TestParseMobaContent_FolderToTags(t *testing.T) {
	content := `[Bookmarks_1]
SubRep=Production
ImgNum=41
WebServer=#109#0%10.0.0.3%22%root%%-1%-1%%%%%0%
DBServer=#109#0%10.0.0.4%22%postgres%%-1%-1%%%%%0%
`
	sessions, err := ParseMobaContent(content)
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 2 {
		t.Fatalf("got %d sessions, want 2", len(sessions))
	}
	for _, s := range sessions {
		if len(s.Tags) != 1 || s.Tags[0] != "production" {
			t.Errorf("session %s tags: got %v, want [production]", s.Name, s.Tags)
		}
	}
}

func TestParseMobaContent_NestedFolders(t *testing.T) {
	content := `[Bookmarks_2]
SubRep=Production\WebServers
ImgNum=41
Web1=#109#0%10.0.0.5%22%root%%-1%-1%%%%%0%
`
	sessions, err := ParseMobaContent(content)
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 {
		t.Fatalf("got %d sessions, want 1", len(sessions))
	}
	s := sessions[0]
	if len(s.Tags) != 2 || s.Tags[0] != "production" || s.Tags[1] != "webservers" {
		t.Errorf("tags: got %v, want [production webservers]", s.Tags)
	}
}

func TestParseMobaContent_SkipNonSSH(t *testing.T) {
	content := `[Bookmarks]
SubRep=
ImgNum=42
SSHServer=#109#0%10.0.0.1%22%root%%-1%
TelnetServer=#98#7%10.0.0.2%23%23%23%0%
RDPServer=#91#3%10.0.0.3%3389%
`
	sessions, err := ParseMobaContent(content)
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 {
		t.Fatalf("got %d sessions, want 1 (only SSH)", len(sessions))
	}
	if sessions[0].Name != "SSHServer" {
		t.Errorf("expected SSHServer, got %s", sessions[0].Name)
	}
}

func TestParseMobaContent_EmptyFile(t *testing.T) {
	sessions, err := ParseMobaContent("")
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 0 {
		t.Errorf("got %d sessions, want 0", len(sessions))
	}
}

func TestParseMobaContent_DefaultUser(t *testing.T) {
	content := `[Bookmarks]
SubRep=
ImgNum=42
NoUser=#109#0%10.0.0.1%22%<default>%%-1%
`
	sessions, err := ParseMobaContent(content)
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 {
		t.Fatalf("got %d sessions, want 1", len(sessions))
	}
	if sessions[0].User != "" {
		t.Errorf("user: got %q, want empty (default)", sessions[0].User)
	}
}

func TestParseMobaContent_MultipleFolders(t *testing.T) {
	content := `[Bookmarks]
SubRep=
ImgNum=42
Root1=#109#0%10.0.0.1%22%root%%-1%

[Bookmarks_1]
SubRep=Web
ImgNum=41
Web1=#109#0%10.0.0.2%22%root%%-1%

[Bookmarks_2]
SubRep=DB
ImgNum=41
DB1=#109#0%10.0.0.3%22%postgres%%-1%
`
	sessions, err := ParseMobaContent(content)
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 3 {
		t.Fatalf("got %d sessions, want 3", len(sessions))
	}
	if len(sessions[0].Tags) != 0 {
		t.Errorf("Root1 tags: got %v, want empty", sessions[0].Tags)
	}
	if len(sessions[1].Tags) != 1 || sessions[1].Tags[0] != "web" {
		t.Errorf("Web1 tags: got %v, want [web]", sessions[1].Tags)
	}
	if len(sessions[2].Tags) != 1 || sessions[2].Tags[0] != "db" {
		t.Errorf("DB1 tags: got %v, want [db]", sessions[2].Tags)
	}
}

func TestFolderToTags(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"", nil},
		{"Production", []string{"production"}},
		{`Production\WebServers`, []string{"production", "webservers"}},
		{`A\B\C`, []string{"a", "b", "c"}},
	}
	for _, tt := range tests {
		got := folderToTags(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("folderToTags(%q) = %v, want %v", tt.input, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("folderToTags(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

func TestParseMobaContent_LogoutPrefix(t *testing.T) {
	content := `[Bookmarks]
SubRep=
ImgNum=42
Server1=; logout#109#0%10.0.0.1%22%root%%-1%
`
	sessions, err := ParseMobaContent(content)
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 {
		t.Fatalf("got %d, want 1", len(sessions))
	}
	if sessions[0].Address != "10.0.0.1" {
		t.Errorf("address: got %q, want 10.0.0.1", sessions[0].Address)
	}
}
