package config

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func setTestHome(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("HOME", dir)
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", dir)
		t.Setenv("HOMEDRIVE", "")
		t.Setenv("HOMEPATH", "")
	}
}

func TestDefaults(t *testing.T) {
	cfg := Defaults()
	if cfg.KeyTTLHours != DefaultKeyTTLHours {
		t.Errorf("KeyTTLHours: got %d, want %d", cfg.KeyTTLHours, DefaultKeyTTLHours)
	}
	if cfg.AuthTimeoutMinutes != DefaultAuthTimeoutMin {
		t.Errorf("AuthTimeoutMinutes: got %d, want %d", cfg.AuthTimeoutMinutes, DefaultAuthTimeoutMin)
	}
	if len(cfg.KeyFormats) != 3 {
		t.Errorf("KeyFormats length: got %d, want 3", len(cfg.KeyFormats))
	}
	if cfg.MobaMaxSessions != 20 {
		t.Errorf("MobaMaxSessions: got %d, want 20", cfg.MobaMaxSessions)
	}
	// Company-specific fields are empty by default
	if cfg.AuthProfile != "" || len(cfg.AuthProfiles) != 0 {
		t.Error("authentication profiles should be empty by default")
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
		{"auth_profile", "primary", func() bool { return cfg.AuthProfile == "primary" }},
		{"auth_profiles.primary.tenant_id", "EXAMPLE", func() bool { return cfg.AuthProfiles["primary"].TenantID == "EXAMPLE" }},
		{"auth_timeout_minutes", "5", func() bool { return cfg.AuthTimeoutMinutes == 5 }},
		{"proxy_host", "proxy.example.com", func() bool { return cfg.ProxyHost == "proxy.example.com" }},
		{"key_dir", "/tmp/keys", func() bool { return cfg.KeyDir == "/tmp/keys" }},
		{"default_ssh_user", "admin", func() bool { return cfg.DefaultSSHUser == "admin" }},
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
	if err := cfg.Set("key_formats", "pem, openssh, pem"); err != nil {
		t.Fatalf("Set key_formats error: %v", err)
	}
	if len(cfg.KeyFormats) != 2 || cfg.KeyFormats[0] != "PEM" || cfg.KeyFormats[1] != "OpenSSH" {
		t.Errorf("key_formats: got %v, want [PEM OpenSSH]", cfg.KeyFormats)
	}
}

func TestValidate(t *testing.T) {
	cfg := Defaults()
	cfg.Username = "mario.rossi"
	cfg.AuthProfile = "primary"
	cfg.AuthProfiles["primary"] = testProfile()
	cfg.ProxyHost = "psmp.example.com"
	cfg.DefaultSSHUser = "root"
	cfg.SSHKeyName = "id_sogark"

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error: %v", err)
	}
}

func TestValidate_InvalidConfig(t *testing.T) {
	cfg := Defaults()
	cfg.SSHKeyName = "nested/path"
	cfg.KeyFormats = []string{"bad"}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() should fail")
	}

	want := []string{"username", "auth_profile", "auth_profiles", "proxy_host", "default_ssh_user", "ssh_key_name", "key_formats"}
	for _, item := range want {
		if !strings.Contains(err.Error(), item) {
			t.Errorf("Validate() error missing %q: %v", item, err)
		}
	}
}

func TestLoadOrDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	setTestHome(t, tmpDir)

	cfg, err := LoadOrDefaults()
	if err != nil {
		t.Fatalf("LoadOrDefaults() error: %v", err)
	}
	if cfg.UpdateRepo != DefaultUpdateRepo {
		t.Fatalf("UpdateRepo: got %q, want %q", cfg.UpdateRepo, DefaultUpdateRepo)
	}
}

func TestSet_InvalidKey(t *testing.T) {
	cfg := Defaults()
	err := cfg.Set("nonexistent", "value")
	if err == nil {
		t.Error("Set with invalid key should return error")
	}
	if !strings.Contains(err.Error(), "unknown key") {
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
	setTestHome(t, tmpDir)

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
	setTestHome(t, tmpDir)

	_, err := Load()
	if err == nil {
		t.Error("Load() should return error when config doesn't exist")
	}
	if !strings.Contains(err.Error(), "configuration not found") {
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
	cfg.AuthProfile = "primary"
	cfg.AuthProfiles["primary"] = testProfile()
	cfg.ProxyHost = "psmp.example.com"
	cfg.DefaultSSHUser = "root"
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

func TestShow_ProfileEndpoints(t *testing.T) {
	cfg := Defaults()
	cfg.AuthProfile = "primary"
	cfg.AuthProfiles["primary"] = testProfile()
	output := cfg.Show()
	if !strings.Contains(output, "saml_bootstrap_url") || !strings.Contains(output, cfg.AuthProfiles["primary"].SAMLBootstrapURL) {
		t.Error("Show() should display the profile endpoints")
	}
}

func testProfile() AuthProfile {
	return AuthProfile{
		AuthType: "saml", TenantID: "EXAMPLE",
		PVWABaseURL:              "https://cyberark.example.com/vault",
		StartAuthenticationURL:   "https://identity.example.com/start",
		AdvanceAuthenticationURL: "https://identity.example.com/advance",
		SAMLBootstrapURL:         "https://cyberark.example.com/bootstrap",
		SAMLLogonURL:             "https://cyberark.example.com/logon",
		SSHKeysCacheURL:          "https://cyberark.example.com/cache",
	}
}

func TestSelectedProfileValidation(t *testing.T) {
	for _, change := range []func(*AuthProfile){
		func(p *AuthProfile) { p.TenantID = "" },
		func(p *AuthProfile) { p.AuthType = "auto" },
		func(p *AuthProfile) { p.StartAuthenticationURL = "http://identity.example.com/start" },
		func(p *AuthProfile) { p.SAMLBootstrapURL = "https://other.example.com/bootstrap" },
		func(p *AuthProfile) {
			p.AdvanceAuthenticationURL = "https://identity.example.com/advance?state=private"
		},
		func(p *AuthProfile) { p.SAMLLogonURL = "https://user:password@cyberark.example.com/logon" },
	} {
		p := testProfile()
		change(&p)
		if err := p.Validate(); err == nil {
			t.Fatal("expected invalid profile error")
		}
	}
	cfg := Defaults()
	cfg.AuthProfile = "missing"
	cfg.AuthProfiles["primary"] = testProfile()
	if _, err := cfg.SelectedAuthProfile(); err == nil {
		t.Fatal("must not fall back to another profile")
	}
}

func TestLegacyConfigurationIsPreserved(t *testing.T) {
	setTestHome(t, t.TempDir())
	path, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	for _, legacy := range []string{"idp_url", "identity_url", "pvwa_base_url"} {
		data := []byte("username: user\n" + legacy + ": https://example.com\n")
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(); !errors.Is(err, ErrLegacyAuthConfig) {
			t.Fatalf("expected explicit migration error for %s, got %v", legacy, err)
		}
		after, err := os.ReadFile(path)
		if err != nil || string(after) != string(data) {
			t.Fatal("loading legacy config must not change it")
		}
	}
}

func TestFileOverrideLoadAndSave(t *testing.T) {
	setTestHome(t, t.TempDir())
	original, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	cfg := Defaults()
	cfg.Username = "original"
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	FileOverride = filepath.Join(t.TempDir(), "private.yaml")
	t.Cleanup(func() { FileOverride = "" })
	cfg.Username = "candidate"
	cfg.AuthProfile = "primary"
	cfg.AuthProfiles["primary"] = testProfile()
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load()
	if err != nil || loaded.Username != "candidate" || len(loaded.AuthProfiles) != 1 {
		t.Fatalf("override round trip failed: %v", err)
	}
	data, err := os.ReadFile(original)
	if err != nil || !strings.Contains(string(data), "original") || strings.Contains(string(data), "candidate") {
		t.Fatal("the default configuration was changed")
	}
	info, err := os.Stat(FileOverride)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatalf("config permissions = %o", info.Mode().Perm())
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
	setTestHome(t, tmpDir)

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
	setTestHome(t, tmpDir)

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
