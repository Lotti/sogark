package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Lotti/sogark/internal/config"
	msg "github.com/Lotti/sogark/internal/messages"
	"github.com/spf13/cobra"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: msg.ConfigShort,
	}

	cmd.AddCommand(
		newConfigInitCmd(),
		newConfigSetCmd(),
		newConfigShowCmd(),
		newConfigWezTermCmd(),
	)

	return cmd
}

func newConfigInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: msg.ConfigInitShort,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			// Start from existing config or defaults
			cfg := config.Defaults()
			existing, loadErr := config.Load()
			legacy := errors.Is(loadErr, config.ErrLegacyAuthConfig)
			if loadErr == nil {
				cfg = *existing
			} else if legacy {
				if existing != nil {
					cfg = *existing
				}
				fmt.Fprintln(cmd.OutOrStdout(), "[i] Rebuilding the authentication profiles; existing non-authentication settings are retained.")
			} else if !errors.Is(loadErr, config.ErrConfigNotFound) {
				return loadErr
			}

			fmt.Fprintln(cmd.OutOrStdout(), msg.ConfigInitTitle)
			fmt.Fprintln(cmd.OutOrStdout(), "─────────────────────")

			prompter, err := newPrompter(cmd.InOrStdin(), cmd.OutOrStdout())
			if err != nil {
				return err
			}
			defer func() {
				err = errors.Join(err, prompter.Close())
			}()
			ask := func(label, current string) (string, error) {
				value, err := prompter.Prompt(label, current)
				if err != nil {
					return "", fmt.Errorf("configuration wizard interrupted; existing configuration unchanged: %w", err)
				}
				return value, nil
			}

			cfg.Username, err = ask(msg.ConfigInitUsername, cfg.Username)
			if err != nil {
				return err
			}
			if cfg.AuthProfile == "" {
				cfg.AuthProfile = "primary"
			}
			cfg.AuthProfile, err = ask("Authentication profile name", cfg.AuthProfile)
			if err != nil {
				return err
			}
			profile := cfg.AuthProfiles[cfg.AuthProfile]
			if profile.AuthType == "" {
				profile.AuthType = "saml"
			}
			profile.AuthType, err = ask("Authentication type (saml or oidc)", profile.AuthType)
			if err != nil {
				return err
			}
			profile.AuthType = strings.ToLower(strings.TrimSpace(profile.AuthType))
			if profile.AuthType != "saml" && profile.AuthType != "oidc" {
				return fmt.Errorf("authentication type must be saml or oidc")
			}
			fields := profile.Fields()
			for _, name := range profile.RequiredFields() {
				*fields[name], err = ask(name, *fields[name])
				if err != nil {
					return err
				}
			}
			if cfg.AuthProfiles == nil {
				cfg.AuthProfiles = make(map[string]config.AuthProfile)
			}
			cfg.AuthProfiles[cfg.AuthProfile] = profile
			cfg.ProxyHost, err = ask("Proxy host", cfg.ProxyHost)
			if err != nil {
				return err
			}
			cfg.SSHKeyName, err = ask(msg.ConfigInitSSHKeyName, cfg.SSHKeyName)
			if err != nil {
				return err
			}
			cfg.KeyDir, err = ask(msg.ConfigInitKeyDir, cfg.KeyDir)
			if err != nil {
				return err
			}
			cfg.DefaultSSHUser, err = ask(msg.ConfigInitSSHUser, cfg.DefaultSSHUser)
			if err != nil {
				return err
			}
			cfg.DefaultSCPUser, err = ask(msg.ConfigInitSCPUser, cfg.DefaultSCPUser)
			if err != nil {
				return err
			}
			formatsStr, err := ask(msg.ConfigInitKeyFormats, strings.Join(cfg.KeyFormats, ","))
			if err != nil {
				return err
			}
			if err := prompter.Close(); err != nil {
				return err
			}

			normalizedFormats, err := config.NormalizeKeyFormats(splitCSV(formatsStr))
			if err != nil {
				return err
			}
			cfg.KeyFormats = normalizedFormats

			if err := cfg.Validate(); err != nil {
				return err
			}

			if legacy {
				backup, err := config.Backup()
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "[+] Previous configuration backed up to %s\n", backup)
			}
			if err := cfg.Save(); err != nil {
				return err
			}

			path, _ := config.Path()
			fmt.Fprintf(cmd.OutOrStdout(), msg.ConfigSavedAt, path)
			return nil
		},
	}
}

func newConfigSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: msg.ConfigSetShort,
		Example: `  sogark config set username mario.rossi
  sogark config set default_ssh_user admin
  sogark config set key_dir /opt/keys`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadOrDefaults()
			if err != nil {
				return err
			}
			if err := cfg.Set(args[0], args[1]); err != nil {
				return err
			}
			if err := cfg.Save(); err != nil {
				return err
			}
			fmt.Printf("[+] %s = %s\n", args[0], args[1])
			return nil
		},
	}
}

func newConfigShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: msg.ConfigShowShort,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			fmt.Println(cfg.Show())
			return nil
		},
	}
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func newConfigWezTermCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "wezterm",
		Short: msg.ConfigWeztermShort,
		Long:  msg.ConfigWeztermLong,
		RunE: func(cmd *cobra.Command, args []string) error {
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf(msg.ConfigErrHomeDir, err)
			}
			luaPath := filepath.Join(home, ".wezterm.lua")

			if _, err := os.Stat(luaPath); err == nil {
				fmt.Printf(msg.ConfigWeztermFileExists, luaPath)
				fmt.Println(msg.ConfigWeztermAddLines)
				fmt.Println()
				fmt.Println(msg.ConfigWeztermRenderComment)
				fmt.Println("  prefer_egl = true,")
				fmt.Println(msg.ConfigWeztermOrComment)
				fmt.Println()
				fmt.Println("  -- Clipboard")
				fmt.Println("  keys = {")
				fmt.Println("    { key = 'c', mods = 'CTRL|SHIFT', action = wezterm.action.CopyTo('Clipboard') },")
				fmt.Println("    { key = 'v', mods = 'CTRL|SHIFT', action = wezterm.action.PasteFrom('Clipboard') },")
				fmt.Println("  },")
				return nil
			}

			if err := os.WriteFile(luaPath, []byte(weztermLuaConfig()), 0644); err != nil {
				return fmt.Errorf(msg.ConfigErrWriteLua, luaPath, err)
			}
			fmt.Printf(msg.ConfigWeztermSaved, luaPath)
			fmt.Println(msg.ConfigWeztermEnabled)
			return nil
		},
	}
}

func weztermLuaConfig() string {
	return `local wezterm = require 'wezterm'
return {
  -- Rendering for VM with limited GPU
  -- prefer_egl uses DirectX/ANGLE (faster), if it doesn't work use front_end = "Software"
  prefer_egl = true,

  -- Clipboard (Ctrl+Shift+C / Ctrl+Shift+V)
  keys = {
    { key = 'c', mods = 'CTRL|SHIFT', action = wezterm.action.CopyTo('Clipboard') },
    { key = 'v', mods = 'CTRL|SHIFT', action = wezterm.action.PasteFrom('Clipboard') },
  },
}
`
}
