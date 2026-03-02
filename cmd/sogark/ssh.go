package main

import (
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

func newSSHCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ssh [sogark-flags] [user@]host [ssh-args...]",
		Short: "Connessione SSH via PSMP con autenticazione automatica",
		Long: `Flusso completo: verifica chiave -> autenticazione SAML/MFA se necessaria -> connessione SSH.

Se l'host corrisponde a un nome registrato in hosts.yaml, ne risolve indirizzo e utente.
Tutti i flag ssh standard sono supportati direttamente.

Flag sogark (--dry-run, --force-login, -u, --key-format) devono precedere l'host.`,
		Example: `  sogark ssh 10.1.2.3
  sogark ssh admin@10.1.2.3
  sogark ssh myserver
  sogark ssh 10.1.2.3 -L 8080:localhost:80
  sogark ssh 10.1.2.3 -v -o StrictHostKeyChecking=no
  sogark ssh 10.1.2.3 -D 1080
  sogark ssh --dry-run 10.1.2.3`,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			user, keyFormat, forceLogin, dryRun, host, sshExtraArgs, err := parseSSHFlags(args)
			if err != nil {
				return err
			}
			if host == "" {
				return fmt.Errorf("specificare l'host\nEsempio: sogark ssh 10.1.2.3")
			}

			cfg, err := config.Load()
			if err != nil {
				return err
			}

			// Resolve from hosts registry
			targetUser, resolvedHost := sshpkg.ParseTarget(host, cfg.DefaultSSHUser)
			if user != "" {
				targetUser = user
			}

			sogarkDir, _ := config.Dir()
			reg, _ := hosts.NewRegistry(sogarkDir)
			if reg != nil {
				if h, ok := reg.Get(resolvedHost); ok {
					resolvedHost = h.Address
					if h.User != "" && user == "" && !strings.Contains(host, "@") {
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
				Host:       resolvedHost,
				ProxyHost:  cfg.ProxyHost,
				KeyPath:    keyPath,
				ExtraArgs:  sshExtraArgs,
			}

			fmt.Printf("> %s\n", connectArgs.CommandString())

			if dryRun {
				return nil
			}

			return connectArgs.Exec()
		},
	}

	return cmd
}

// parseSSHFlags separates sogark-specific flags from ssh passthrough args.
// Returns: user, keyFormat, forceLogin, dryRun, host, sshExtraArgs, err.
// The first non-flag, non-sogark argument is treated as the host.
func parseSSHFlags(args []string) (user, keyFormat string, forceLogin, dryRun bool, host string, sshArgs []string, err error) {
	keyFormat = "openssh"
	i := 0
	hostFound := false
	for i < len(args) {
		a := args[i]
		switch {
		case a == "--verbose":
			os.Setenv("SOGARK_DEBUG", "1")
		case !hostFound && a == "--dry-run":
			dryRun = true
		case !hostFound && a == "--force-login":
			forceLogin = true
		case !hostFound && (a == "-u" || a == "--user"):
			i++
			if i >= len(args) {
				err = fmt.Errorf("flag %s richiede un valore", a)
				return
			}
			user = args[i]
		case !hostFound && strings.HasPrefix(a, "--user="):
			user = strings.TrimPrefix(a, "--user=")
		case !hostFound && a == "--key-format":
			i++
			if i >= len(args) {
				err = fmt.Errorf("flag %s richiede un valore", a)
				return
			}
			keyFormat = args[i]
		case !hostFound && strings.HasPrefix(a, "--key-format="):
			keyFormat = strings.TrimPrefix(a, "--key-format=")
		case a == "-h" || a == "--help":
			err = fmt.Errorf("help")
			return
		case !hostFound && a == "--":
			// skip separator, next non-flag is host
		case !hostFound && !strings.HasPrefix(a, "-"):
			host = a
			hostFound = true
		default:
			sshArgs = append(sshArgs, a)
		}
		i++
	}
	return
}

// doLogin performs the full SAML login + key fetch flow.
func doLogin(cfg *config.Config) error {
	samlResponse, err := auth.SAMLResponse(signalCtx, cfg.IDPURL, cfg.SAMLTimeoutMinutes)
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
