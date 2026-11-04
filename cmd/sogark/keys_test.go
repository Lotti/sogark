package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Lotti/sogark/internal/config"
	"github.com/Lotti/sogark/internal/keys"
)

func TestSSHAndSCPReuseSelectedPEMWithoutOpenSSH(t *testing.T) {
	for _, command := range []string{"ssh", "scp"} {
		t.Run(command, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("USERPROFILE", home)
			isolatedConfigPath(t, "")
			cfg := config.Defaults()
			cfg.AuthProfile = "test"
			cfg.Username = "user"
			cfg.ProxyHost = "proxy.example.com"
			cfg.DefaultSSHUser = "root"
			cfg.KeyDir = t.TempDir()
			cfg.SSHKeyName = "testkey"
			cfg.KeyFormats = []string{"PEM"}
			if err := cfg.Save(); err != nil {
				t.Fatal(err)
			}
			if _, err := keys.Save(&keys.Parsed{PEM: "pem-fixture"}, cfg.KeyDir, cfg.SSHKeyName, cfg.KeyFormats); err != nil {
				t.Fatal(err)
			}
			if err := keys.SaveTimestamp(cfg.KeyDir); err != nil {
				t.Fatal(err)
			}
			cmd := newSSHCmd()
			args := []string{"--key-format", "pem", "--dry-run", "host"}
			if command == "scp" {
				cmd = newScpCmd()
				args = []string{"--key-format", "pem", "--dry-run", "local-file", "host:/tmp/"}
			}
			cmd.SetArgs(args)
			if err := cmd.Execute(); err != nil {
				t.Fatalf("selected PEM cache should avoid login: %v", err)
			}
		})
	}
}

func TestKeysUsesSelectedDirectoryAndFormats(t *testing.T) {
	for _, tt := range []struct {
		name       string
		cached     []string
		args       []string
		wantFormat []string
		wantLogin  bool
		wantError  bool
	}{
		{name: "empty override directory", wantFormat: []string{"OpenSSH"}, wantLogin: true},
		{name: "cached override directory", cached: []string{"OpenSSH"}, wantFormat: []string{"OpenSSH"}},
		{name: "missing PEM", cached: []string{"OpenSSH"}, args: []string{"--format", "pem"}, wantFormat: []string{"PEM"}, wantLogin: true},
		{name: "PEM without OpenSSH", cached: []string{"PEM"}, args: []string{"--format", "pEm"}, wantFormat: []string{"PEM"}},
		{name: "PPK without OpenSSH", cached: []string{"PPK"}, args: []string{"--format", "ppk"}, wantFormat: []string{"PPK"}},
		{name: "forced login", cached: []string{"OpenSSH"}, args: []string{"--force-login"}, wantFormat: []string{"OpenSSH"}, wantLogin: true},
		{name: "invalid format with valid cache", cached: []string{"OpenSSH"}, args: []string{"--format", "invalid"}, wantError: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			path := isolatedConfigPath(t, "")
			cfg := config.Defaults()
			cfg.AuthProfile = "test"
			cfg.KeyDir = t.TempDir()
			cfg.SSHKeyName = "testkey"
			cfg.KeyFormats = []string{"OpenSSH"}
			if err := cfg.Save(); err != nil {
				t.Fatal(err)
			}
			originalConfig, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			originalKey := filepath.Join(cfg.KeyDir, cfg.SSHKeyName)
			if err := os.WriteFile(originalKey, []byte("operational-key-fixture"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := keys.SaveTimestamp(cfg.KeyDir); err != nil {
				t.Fatal(err)
			}
			originalTimestamp, err := os.ReadFile(filepath.Join(cfg.KeyDir, ".key_timestamp"))
			if err != nil {
				t.Fatal(err)
			}

			dir := t.TempDir()
			parsed := &keys.Parsed{OpenSSH: "openssh-fixture", PEM: "pem-fixture", PPK: "ppk-fixture"}
			if len(tt.cached) > 0 {
				if _, err := keys.Save(parsed, dir, cfg.SSHKeyName, tt.cached); err != nil {
					t.Fatal(err)
				}
				if err := keys.SaveTimestamp(dir); err != nil {
					t.Fatal(err)
				}
			}
			logins := 0
			cmd := newKeysCmdWithLogin(func(selected *config.Config) error {
				logins++
				if selected.KeyDir != dir || !reflect.DeepEqual(selected.KeyFormats, tt.wantFormat) {
					t.Fatalf("login received wrong destination or formats: %q, %v", selected.KeyDir, selected.KeyFormats)
				}
				if _, err := keys.Save(parsed, selected.KeyDir, selected.SSHKeyName, selected.KeyFormats); err != nil {
					return err
				}
				return keys.SaveTimestamp(selected.KeyDir)
			})
			cmd.SilenceErrors, cmd.SilenceUsage = true, true
			cmd.SetArgs(append([]string{"--dir", dir}, tt.args...))
			err = cmd.Execute()
			if (err != nil) != tt.wantError {
				t.Fatalf("unexpected command result: %v", err)
			}
			if (logins == 1) != tt.wantLogin || logins > 1 {
				t.Fatalf("login count = %d, want login = %v", logins, tt.wantLogin)
			}
			if !tt.wantError {
				valid, _, err := keys.IsValid(dir, cfg.SSHKeyName, cfg.KeyTTLHours, tt.wantFormat...)
				if err != nil || !valid {
					t.Fatalf("selected keys were not saved in override directory: valid=%v, error=%v", valid, err)
				}
			}
			for filename, want := range map[string][]byte{
				path: originalConfig, originalKey: []byte("operational-key-fixture"),
				filepath.Join(cfg.KeyDir, ".key_timestamp"): originalTimestamp,
			} {
				got, err := os.ReadFile(filename)
				if err != nil || !bytes.Equal(got, want) {
					t.Fatalf("original file was changed: %s, error=%v", filename, err)
				}
			}
		})
	}
}
