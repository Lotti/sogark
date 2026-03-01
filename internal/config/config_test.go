package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaults(t *testing.T) {
	cfg := Defaults()
	if cfg.KeyTTLHours != DefaultKeyTTLHours {
		t.Errorf("KeyTTLHours: got %d, want %d", cfg.KeyTTLHours, DefaultKeyTTLHours)
	}
	if cfg.SAMLTimeoutMinutes != DefaultSAMLTimeoutMin {
		t.Errorf("SAMLTimeoutMinutes: got %d, want %d", cfg.SAMLTimeoutMinutes, DefaultSAMLTimeoutMin)
	}
	if len(cfg.KeyFormats) != 3 {
		t.Errorf("KeyFormats length: got %d, want 3", len(cfg.KeyFormats))
	}
	if cfg.MobaMaxSessions != 20 {
		t.Errorf("MobaMaxSessions: got %d, want 20", cfg.MobaMaxSessions)
	}
	// Company-specific fields are empty by default
	if cfg.PVWABaseURL != "" {
		t.Errorf("PVWABaseURL should be empty by default, got %q", cfg.PVWABaseURL)
	}
	if cfg.ProxyHost != "" {
		t.Errorf("ProxyHost should be empty by default, got %q", cfg.ProxyHost)
	}
}

func TestDefaults_KeyFormatsIsCopy(t *testing.T) {
	cfg1 := Defaults()
	cfg2 := Defaults()
	cfg1.KeyFormats[0] = "CHANGED"
	if cfg2.KeyFormats[0] == "CHANGED" {
		t.Error("Defaults() should return independent copies of KeyFormats")
	}
}

func TestSet_ValidKeys(t *testing.T) {
	cfg := Defaults()

	tests := []struct {
		key   string
		value string
		check func() bool
	}{
		{"username", "mario.rossi", func() bool { return cfg.Username == "mario.rossi" }},
		{"pvwa_base_url", "https://example.com", func() bool { return cfg.PVWABaseURL == "https://example.com" }},
		{"idp_url", "https://idp.example.com", func() bool { return cfg.IDPURL == "https://idp.example.com" }},
		{"proxy_host", "proxy.example.com", func() bool { return cfg.ProxyHost == "proxy.example.com" }},
		{"key_dir", "/tmp/keys", func() bool { return cfg.KeyDir == "/tmp/keys" }},
		{"default_target_user", "admin", func() bool { return cfg.DefaultTargetUser == "admin" }},
		{"ssh_key_name", "my_key", func() bool { return cfg.SSHKeyName == "my_key" }},
		{"key_ttl_hours", "8", func() bool { return cfg.KeyTTLHours == 8 }},
		{"moba_path", `C:\Tools\MobaXterm.exe`, func() bool { return cfg.MobaPath == `C:\Tools\MobaXterm.exe` }},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			if err := cfg.Set(tt.key, tt.value); err != nil {
				t.Fatalf("Set(%q, %q) error: %v", tt.key, tt.value, err)
			}
			if !tt.check() {
				t.Errorf("Set(%q, %q) did not update correctly", tt.key, tt.value)
			}
		})
	}
}

func TestSet_KeyFormats(t *testing.T) {
	cfg := Defaults()
	if err := cfg.Set("key_formats", "PEM, OpenSSH"); err != nil {
		t.Fatalf("Set key_formats error: %v", err)
	}
	if len(cfg.KeyFormats) != 2 || cfg.KeyFormats[0] != "PEM" || cfg.KeyFormats[1] != "OpenSSH" {
		t.Errorf("key_formats: got %v, want [PEM OpenSSH]", cfg.KeyFormats)
	}
}

func TestSet_InvalidKey(t *testing.T) {
	cfg := Defaults()
	err := cfg.Set("nonexistent", "value")
	if err == nil {
		t.Error("Set with invalid key should return error")
	}
	if !strings.Contains(err.Error(), "chiave sconosciuta") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestSet_KeyTTLHours_Invalid(t *testing.T) {
	cfg := Defaults()

	for _, val := range []string{"abc", "0", "-1", ""} {
		if err := cfg.Set("key_ttl_hours", val); err == nil {
			t.Errorf("Set key_ttl_hours=%q should return error", val)
		}
	}
}

func TestSaveAndLoad(t *testing.T) {
	// Use a temp dir as HOME
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	cfg := Defaults()
	cfg.Username = "test.user"
	cfg.KeyDir = filepath.Join(tmpDir, DirName, KeysDirName)

	if err := cfg.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if loaded.Username != "test.user" {
		t.Errorf("Username: got %q, want %q", loaded.Username, "test.user")
	}
	if loaded.KeyTTLHours != DefaultKeyTTLHours {
		t.Errorf("KeyTTLHours: got %d, want %d", loaded.KeyTTLHours, DefaultKeyTTLHours)
	}
}

func TestLoad_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	_, err := Load()
	if err == nil {
		t.Error("Load() should return error when config doesn't exist")
	}
	if !strings.Contains(err.Error(), "configurazione non trovata") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestResolveKeyDir(t *testing.T) {
	cfg := Config{KeyDir: "/absolute/path/keys"}
	dir, err := cfg.ResolveKeyDir()
	if err != nil {
		t.Fatalf("ResolveKeyDir error: %v", err)
	}
	if dir != "/absolute/path/keys" {
		t.Errorf("got %q, want /absolute/path/keys", dir)
	}
}

func TestResolveKeyDir_Tilde(t *testing.T) {
	cfg := Config{KeyDir: "~/mykeys"}
	dir, err := cfg.ResolveKeyDir()
	if err != nil {
		t.Fatalf("ResolveKeyDir error: %v", err)
	}
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, "mykeys")
	if dir != expected {
		t.Errorf("got %q, want %q", dir, expected)
	}
}

func TestShow(t *testing.T) {
	cfg := Defaults()
	cfg.Username = "mario.rossi"
	cfg.PVWABaseURL = "https://cyberark.example.com/PasswordVault"
	cfg.ProxyHost = "psmp.example.com"
	cfg.DefaultTargetUser = "root"
	cfg.SSHKeyName = "id_example"
	output := cfg.Show()

	mustContain := []string{
		"mario.rossi",
		"https://cyberark.example.com",
		"psmp.example.com",
		"root",
		"id_example",
		"4",
	}
	for _, s := range mustContain {
		if !strings.Contains(output, s) {
			t.Errorf("Show() output missing %q", s)
		}
	}
}

func TestShow_LongIDPURLTruncated(t *testing.T) {
	cfg := Defaults()
	cfg.IDPURL = "https://idp.example.com/login?param1=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa&param2=bbbbbbb"
	output := cfg.Show()
	// Long IDP URL should be truncated with "..."
	if !strings.Contains(output, "...") {
		t.Error("Show() should truncate long IDP URLs")
	}
}

func TestShow_MobaPath(t *testing.T) {
	cfg := Defaults()
	// Without moba_path, it should not appear
	output := cfg.Show()
	if strings.Contains(output, "moba_path") {
		t.Error("Show() should not include moba_path when empty")
	}
	// With moba_path, it should appear
	cfg.MobaPath = `C:\Tools\MobaXterm.exe`
	output = cfg.Show()
	if !strings.Contains(output, `C:\Tools\MobaXterm.exe`) {
		t.Error("Show() should include moba_path when set")
	}
	if !strings.Contains(output, "moba_path") {
		t.Error("Show() should include moba_path label")
	}
}

func TestSaveAndLoad_MobaPath(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	cfg := Defaults()
	cfg.Username = "test"
	cfg.MobaPath = `C:\MobaXterm.exe`
	cfg.KeyDir = filepath.Join(tmpDir, DirName, KeysDirName)

	if err := cfg.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if loaded.MobaPath != `C:\MobaXterm.exe` {
		t.Errorf("MobaPath: got %q, want %q", loaded.MobaPath, `C:\MobaXterm.exe`)
	}
}

func TestSplitAndTrim(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"a,b,c", []string{"a", "b", "c"}},
		{" a , b , c ", []string{"a", "b", "c"}},
		{"a,,b", []string{"a", "b"}},
		{"", nil},
		{"single", []string{"single"}},
	}
	for _, tt := range tests {
		result := splitAndTrim(tt.input)
		if len(result) != len(tt.expected) {
			t.Errorf("splitAndTrim(%q): got %v, want %v", tt.input, result, tt.expected)
			continue
		}
		for i := range result {
			if result[i] != tt.expected[i] {
				t.Errorf("splitAndTrim(%q)[%d]: got %q, want %q", tt.input, i, result[i], tt.expected[i])
			}
		}
	}
}

func TestSave_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	cfg := Defaults()
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	sogarkDir := filepath.Join(tmpDir, DirName)
	info, err := os.Stat(sogarkDir)
	if err != nil {
		t.Fatalf("sogark directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("sogark path is not a directory")
	}
}
