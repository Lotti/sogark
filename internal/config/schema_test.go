package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadRejectsMissingOrOldAuthenticationStructure(t *testing.T) {
	previous := FileOverride
	FileOverride = filepath.Join(t.TempDir(), "config.yaml")
	t.Cleanup(func() { FileOverride = previous })
	for _, content := range []string{
		"", "username: old\n", "auth_profile: primary\n", "auth_profiles: {}\n",
		"auth_profile: null\nauth_profiles: null\n",
		"auth_profile: 42\nauth_profiles: []\n",
		"auth_profile: primary\nauth_profiles: wrong\n",
	} {
		if err := os.WriteFile(FileOverride, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(); !errors.Is(err, ErrLegacyAuthConfig) {
			t.Fatalf("unsupported structure must return migration error: %q, %v", content, err)
		}
		data, _ := os.ReadFile(FileOverride)
		if string(data) != content {
			t.Fatal("schema detection changed the file")
		}
	}
}
