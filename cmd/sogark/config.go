package main

import (
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
		RunE: func(cmd *cobra.Command, args []string) error {
			// Start from existing config or defaults
			cfg := config.Defaults()
			if existing, err := config.Load(); err == nil {
				cfg = *existing
			}

			fmt.Println(msg.ConfigInitTitle)
			fmt.Println("─────────────────────")

			prompter := newPrompter(os.Stdin, os.Stdout)
			prompterClosed := false
			defer func() {
				if !prompterClosed {
					_ = prompter.Close()
				}
			}()
			ask := func(label, current string) (string, error) {
				return prompter.Prompt(label, current)
			}
			var err error

			cfg.Username, err = ask(msg.ConfigInitUsername, cfg.Username)
			if err != nil {
				return err
			}
			cfg.PVWABaseURL, err = ask("PVWA Base URL", cfg.PVWABaseURL)
			if err != nil {
				return err
			}
			cfg.IDPURL, err = ask("IDP URL", cfg.IDPURL)
			if err != nil {
				return err
			}
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
			prompterClosed = true

			normalizedFormats, err := config.NormalizeKeyFormats(splitCSV(formatsStr))
			if err != nil {
				return err
			}
			cfg.KeyFormats = normalizedFormats

			if err := cfg.Validate(); err != nil {
				return err
			}

			if err := cfg.Save(); err != nil {
				return err
			}

			path, _ := config.Path()
			fmt.Printf(msg.ConfigSavedAt, path)
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
