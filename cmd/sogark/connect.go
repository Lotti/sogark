package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sogei/cyberark-cli/internal/auth"
	"github.com/sogei/cyberark-cli/internal/config"
	"github.com/sogei/cyberark-cli/internal/hosts"
	"github.com/sogei/cyberark-cli/internal/keys"
	sshpkg "github.com/sogei/cyberark-cli/internal/ssh"
	"github.com/spf13/cobra"
)

func newConnectCmd() *cobra.Command {
	var (
		user       string
		keyFormat  string
		forceLogin bool
		dryRun     bool
	)

	cmd := &cobra.Command{
		Use:   "connect [user@]host [-- ssh-args...]",
		Short: "Connessione SSH via PSMP con autenticazione automatica",
		Long: `Flusso completo: verifica chiave -> autenticazione SAML/MFA se necessaria -> connessione SSH.

Se l'host corrisponde a un nome registrato in hosts.yaml, ne risolve indirizzo e utente.
Argomenti dopo -- vengono passati direttamente al client ssh.`,
		Example: `  sogark connect 10.1.2.3
  sogark connect admin@10.1.2.3
  sogark connect myserver
  sogark connect 10.1.2.3 -- -L 8080:localhost:80`,
		Args:               cobra.MinimumNArgs(1),
		DisableFlagParsing: false,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			target := args[0]
			extraArgs := args[1:]

			// Resolve from hosts registry
			targetUser, host := sshpkg.ParseTarget(target, cfg.DefaultTargetUser)
			if user != "" {
				targetUser = user
			}

			sogarkDir, _ := config.Dir()
			reg, _ := hosts.NewRegistry(sogarkDir)
			if reg != nil {
				if h, ok := reg.Get(host); ok {
					host = h.Address
					if h.User != "" && user == "" && !strings.Contains(target, "@") {
						targetUser = h.User
					}
				}
			}

			keyDir, err := cfg.ResolveKeyDir()
			if err != nil {
				return err
			}

			// Check key validity
			valid, remaining, _ := keys.IsValid(keyDir, cfg.SSHKeyName, cfg.KeyTTLHours)
			if valid && !forceLogin {
				fmt.Printf("[+] Chiave valida (scade tra %s)\n", formatDuration(remaining))
			} else {
				if !valid {
					fmt.Println("[!] Chiave scaduta o assente, avvio autenticazione...")
				}
				if err := doLogin(cfg); err != nil {
					return err
				}
			}

			// Determine key path
			keyName := cfg.SSHKeyName
			if keyFormat == "pem" {
				keyName += ".pem"
			}
			keyPath := filepath.Join(keyDir, keyName)

			connectArgs := &sshpkg.ConnectArgs{
				Username:   cfg.Username,
				TargetUser: targetUser,
				Host:       host,
				ProxyHost:  cfg.ProxyHost,
				KeyPath:    keyPath,
				ExtraArgs:  extraArgs,
			}

			fmt.Printf("> %s\n", connectArgs.CommandString())

			if dryRun {
				return nil
			}

			return connectArgs.Exec()
		},
	}

	cmd.Flags().StringVarP(&user, "user", "u", "", "utente target sulla macchina remota (override)")
	cmd.Flags().StringVar(&keyFormat, "key-format", "openssh", "formato chiave da usare: openssh, pem")
	cmd.Flags().BoolVar(&forceLogin, "force-login", false, "forza ri-autenticazione anche se la chiave è valida")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "mostra il comando SSH senza eseguirlo")

	return cmd
}

// doLogin performs the full SAML login + key fetch flow.
func doLogin(cfg *config.Config) error {
	samlResponse, err := auth.SAMLResponse(context.Background(), cfg.IDPURL)
	if err != nil {
		return err
	}

	client := auth.NewClient(cfg.PVWABaseURL)
	if err := client.Logon(samlResponse); err != nil {
		return err
	}

	raw, err := client.FetchSSHKeys(cfg.KeyFormats)
	if err != nil {
		return err
	}

	parsed, err := keys.Parse(raw)
	if err != nil {
		return err
	}

	keyDir, err := cfg.ResolveKeyDir()
	if err != nil {
		return err
	}

	results, err := keys.Save(parsed, keyDir, cfg.SSHKeyName, cfg.KeyFormats)
	if err != nil {
		return err
	}

	if os.Getenv("SOGARK_DEBUG") != "" {
		for _, r := range results {
			data, _ := os.ReadFile(r.Path)
			fmt.Fprintf(os.Stderr, "[DEBUG] Saved %s (%d bytes), first 100 chars: %q\n", r.Path, len(data), truncate(string(data), 100))
		}
	}

	if err := keys.SaveTimestamp(keyDir); err != nil {
		return err
	}

	fmt.Println("[+] Chiavi salvate:")
	for _, r := range results {
		fmt.Printf("    %-40s (%s)\n", r.Path, r.Format)
	}
	fmt.Printf("  Scadenza: tra %dh\n", cfg.KeyTTLHours)

	return nil
}

func formatDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
