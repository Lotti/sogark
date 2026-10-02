package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Lotti/sogark/internal/config"
	"github.com/spf13/cobra"
)

func isolatedConfigPath(t *testing.T, contents string) string {
	t.Helper()
	previous := config.FileOverride
	path := filepath.Join(t.TempDir(), "config.yaml")
	config.FileOverride = path
	t.Cleanup(func() { config.FileOverride = previous })
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestStartupRejectsOldConfigBeforeCommands(t *testing.T) {
	path := isolatedConfigPath(t, "username: previous\nproxy_host: proxy.example.com\n")
	for _, args := range [][]string{
		{"login"}, {"moba", "host"}, {"ssh", "host"}, {"scp", "file", "host:/tmp/"},
		{"keys", "list"}, {"hosts", "list"}, {"doctor"}, {"config", "show"},
	} {
		t.Run(strings.Join(args, "-"), func(t *testing.T) {
			root := newRootCmd()
			root.SetArgs(append([]string{"--config", path}, args...))
			root.SetOut(io.Discard)
			root.SetErr(io.Discard)
			err := root.Execute()
			if !errors.Is(err, config.ErrLegacyAuthConfig) || !strings.Contains(err.Error(), "config init") {
				t.Fatalf("expected migration instruction before command execution, got %v", err)
			}
		})
	}
}

func TestStartupHelpAndVersionRemainAvailable(t *testing.T) {
	path := isolatedConfigPath(t, "username: old\n")
	for _, args := range [][]string{{"--help"}, {"--version"}, {"ssh", "--help"}, {"scp", "--help"}} {
		root := newRootCmd()
		root.SetArgs(append([]string{"--config", path}, args...))
		root.SetOut(io.Discard)
		root.SetErr(io.Discard)
		if err := root.Execute(); err != nil {
			t.Fatalf("help/version must remain available with legacy config: %v", err)
		}
	}
}

func TestStartupUsesCustomConfigForPassthroughCommands(t *testing.T) {
	for _, command := range []string{"ssh", "scp"} {
		path := isolatedConfigPath(t, `auth_profile: primary
auth_profiles:
  primary:
    auth_type: saml
    tenant_id: EXAMPLE
    pvwa_base_url: https://vault.example.com
    start_authentication_url: https://identity.example.com/start
    advance_authentication_url: https://identity.example.com/advance
    saml_bootstrap_url: https://vault.example.com/bootstrap
    saml_logon_url: https://vault.example.com/logon
    ssh_keys_cache_url: https://vault.example.com/cache
`)
		root := newRootCmd()
		child, _, err := root.Find([]string{command})
		if err != nil {
			t.Fatal(err)
		}

		ran := false
		child.RunE = func(*cobra.Command, []string) error { ran = true; return nil }
		root.SetArgs([]string{command, "--config", path, "host"})
		if err := root.Execute(); err != nil || !ran || config.FileOverride != path {
			t.Fatalf("startup did not use custom configuration for %s: %v", command, err)
		}
	}
}

func TestStartupRejectsIncompleteSelectedProfile(t *testing.T) {
	for _, contents := range []string{
		"auth_profile: ''\nauth_profiles: {}\n",
		"auth_profile: primary\nauth_profiles: {}\n",
		"auth_profile: primary\nauth_profiles:\n  primary:\n    auth_type: saml\n",
	} {
		path := isolatedConfigPath(t, contents)
		for _, args := range [][]string{{"login"}, {"ssh", "host"}, {"scp", "file", "host:/tmp/"}, {"moba", "host"}} {
			root := newRootCmd()
			child, _, err := root.Find(args[:1])
			if err != nil {
				t.Fatal(err)
			}
			ran := false
			child.RunE = func(*cobra.Command, []string) error { ran = true; return nil }
			root.SetArgs(append([]string{"--config", path}, args...))
			root.SetOut(io.Discard)
			err = root.Execute()
			if err == nil || !strings.Contains(err.Error(), "config init") || ran {
				t.Fatalf("an incomplete active profile must fail before starting %s: %v", args[0], err)
			}
		}
	}
}

func TestStartupAllowsEditingAndDiagnosingIncompleteProfiles(t *testing.T) {
	path := isolatedConfigPath(t, "auth_profile: primary\nauth_profiles: {}\n")
	for _, args := range [][]string{
		{"config", "show"}, {"config", "set", "auth_profile", "primary"},
		{"doctor"}, {"completion", "powershell"},
	} {
		root := newRootCmd()
		child, _, err := root.Find(args)
		if err != nil {
			t.Fatal(err)
		}
		ran := false
		child.RunE = func(*cobra.Command, []string) error { ran = true; return nil }
		root.SetArgs(append([]string{"--config", path}, args...))
		if err := root.Execute(); err != nil || !ran {
			t.Fatalf("incomplete profiles must remain editable and diagnosable: %v", err)
		}
	}
}

func TestStartupDoesNotBlockUpdateHelper(t *testing.T) {
	path := isolatedConfigPath(t, "username: old\n")
	root := newRootCmd()
	child, _, err := root.Find([]string{"__complete-update"})
	if err != nil {
		t.Skip("this platform does not need the update helper")
	}
	child.Args = nil
	ran := false
	child.RunE = func(*cobra.Command, []string) error { ran = true; return nil }
	root.SetArgs([]string{"--config", path, "__complete-update"})
	if err := root.Execute(); err != nil || !ran {
		t.Fatalf("legacy config must not prevent completing an already started update: %v", err)
	}
}

func TestConfigWizardMigratesAndPreservesSettings(t *testing.T) {
	original := "username: old.user\nidp_url: https://identity.example.com/login\nproxy_host: proxy.example.com\nssh_key_name: id_existing\nkey_dir: ~/existing keys\ndefault_ssh_user: operator\nkey_ttl_hours: 8\nmoba_path: 'C:\\Tools\\MobaXterm.exe'\n"
	path := isolatedConfigPath(t, original)
	input := "\n primary \n saml \n EXAMPLE \n https://vault.example.com/vault \n https://identity.example.com/start \n https://identity.example.com/advance \n https://vault.example.com/cache \n https://vault.example.com/bootstrap \n https://vault.example.com/logon \n\n\n\n\n\n\n"
	root := newRootCmd()
	root.SetArgs([]string{"--config", path, "config", "init"})
	root.SetIn(strings.NewReader(input))
	var output bytes.Buffer
	root.SetOut(&output)
	if err := root.Execute(); err != nil {
		t.Fatalf("legacy configuration must be recoverable through the wizard: %v", err)
	}
	cfg, err := config.Load()
	if err != nil || cfg.AuthProfile != "primary" || cfg.AuthProfiles["primary"].TenantID != "EXAMPLE" {
		t.Fatalf("new profile was not saved: %v", err)
	}
	if cfg.Username != "old.user" || cfg.ProxyHost != "proxy.example.com" || cfg.SSHKeyName != "id_existing" ||
		cfg.KeyDir != "~/existing keys" || cfg.KeyTTLHours != 8 || cfg.MobaPath != `C:\Tools\MobaXterm.exe` {
		t.Fatal("migration changed existing non-authentication settings")
	}
	backups, err := filepath.Glob(path + ".legacy-*.bak")
	if err != nil || len(backups) != 1 {
		t.Fatalf("expected one legacy backup: %v, %v", backups, err)
	}
	data, err := os.ReadFile(backups[0])
	if err != nil || string(data) != original {
		t.Fatal("legacy backup must preserve the exact original contents")
	}
}

func TestConfigWizardCancellationLeavesOriginalUntouched(t *testing.T) {
	original := "username: old.user\nidp_url: https://identity.example.com/login\n"
	path := isolatedConfigPath(t, original)
	root := newRootCmd()
	root.SetArgs([]string{"--config", path, "config", "init"})
	root.SetIn(strings.NewReader(""))
	root.SetOut(io.Discard)
	if err := root.Execute(); !errors.Is(err, io.EOF) {
		t.Fatalf("wizard must stop on EOF instead of silently accepting defaults: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != original {
		t.Fatal("cancelled wizard changed the original configuration")
	}
	backups, _ := filepath.Glob(path + ".legacy-*.bak")
	if len(backups) != 0 {
		t.Fatal("cancelled wizard must not create a migration backup")
	}
}
