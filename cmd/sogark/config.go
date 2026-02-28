package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sogei/cyberark-cli/internal/config"
	"github.com/spf13/cobra"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Gestione configurazione sogark",
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
		Short: "Wizard interattivo per la prima configurazione",
		RunE: func(cmd *cobra.Command, args []string) error {
			reader := bufio.NewReader(os.Stdin)

			// Start from existing config or defaults
			cfg := config.Defaults()
			if existing, err := config.Load(); err == nil {
				cfg = *existing
			}

			fmt.Println("Configurazione sogark")
			fmt.Println("─────────────────────")

			cfg.Username = prompt(reader, "Username aziendale", cfg.Username)
			cfg.PVWABaseURL = prompt(reader, "PVWA Base URL", cfg.PVWABaseURL)
			cfg.IDPURL = prompt(reader, "IDP URL", cfg.IDPURL)
			cfg.ProxyHost = prompt(reader, "Proxy host", cfg.ProxyHost)
			cfg.KeyDir = prompt(reader, "Directory chiavi", cfg.KeyDir)
			cfg.DefaultTargetUser = prompt(reader, "Utente target di default", cfg.DefaultTargetUser)
			formatsStr := prompt(reader, "Formati chiave", strings.Join(cfg.KeyFormats, ","))
			cfg.KeyFormats = splitCSV(formatsStr)

			if err := cfg.Save(); err != nil {
				return err
			}

			path, _ := config.Path()
			fmt.Printf("\n[+] Configurazione salvata in %s\n", path)
			return nil
		},
	}
}

func newConfigSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Imposta un parametro di configurazione",
		Example: `  sogark config set username mario.rossi
  sogark config set default_target_user admin
  sogark config set key_dir /opt/keys`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
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
		Short: "Mostra la configurazione corrente",
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

func prompt(reader *bufio.Reader, label, defaultVal string) string {
	if defaultVal != "" {
		fmt.Printf("%s [%s]: ", label, defaultVal)
	} else {
		fmt.Printf("%s: ", label)
	}
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input == "" {
		return defaultVal
	}
	return input
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
		Short: "Genera il file di configurazione WezTerm per VM",
		Long: `Genera ~/.wezterm.lua con rendering software (per VM con GPU limitata)
e supporto clipboard (Ctrl+Shift+C/V).
Se il file esiste già, stampa le istruzioni per la configurazione manuale.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("impossibile determinare la home directory: %w", err)
			}
			luaPath := filepath.Join(home, ".wezterm.lua")

			if _, err := os.Stat(luaPath); err == nil {
				fmt.Printf("[i] Il file %s esiste già.\n", luaPath)
				fmt.Println("    Aggiungi manualmente queste righe alla tua configurazione:")
				fmt.Println()
				fmt.Println("  -- Rendering software per VM con GPU limitata")
				fmt.Println("  front_end = \"Software\",")
				fmt.Println()
				fmt.Println("  -- Clipboard")
				fmt.Println("  keys = {")
				fmt.Println("    { key = 'c', mods = 'CTRL|SHIFT', action = wezterm.action.CopyTo('Clipboard') },")
				fmt.Println("    { key = 'v', mods = 'CTRL|SHIFT', action = wezterm.action.PasteFrom('Clipboard') },")
				fmt.Println("  },")
				return nil
			}

			if err := os.WriteFile(luaPath, []byte(weztermLuaConfig()), 0644); err != nil {
				return fmt.Errorf("errore scrittura %s: %w", luaPath, err)
			}
			fmt.Printf("[+] Configurazione WezTerm salvata in %s\n", luaPath)
			fmt.Println("    Rendering software + clipboard abilitati.")
			return nil
		},
	}
}

func weztermLuaConfig() string {
	return `local wezterm = require 'wezterm'
return {
  -- Rendering software per VM con GPU limitata
  front_end = "Software",

  -- Clipboard (Ctrl+Shift+C / Ctrl+Shift+V)
  keys = {
    { key = 'c', mods = 'CTRL|SHIFT', action = wezterm.action.CopyTo('Clipboard') },
    { key = 'v', mods = 'CTRL|SHIFT', action = wezterm.action.PasteFrom('Clipboard') },
  },
}
`
}
