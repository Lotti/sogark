package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sogei/cyberark-cli/internal/config"
	"github.com/sogei/cyberark-cli/internal/hosts"
	"github.com/sogei/cyberark-cli/internal/keys"
	sshpkg "github.com/sogei/cyberark-cli/internal/ssh"
	"github.com/spf13/cobra"
)

func newScpCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scp [sogark-flags] source... target",
		Short: "Trasferimento file via SCP attraverso PSMP",
		Long: `Wrapper trasparente per scp: sogark inietta la chiave SSH (-i) e traduce i path remoti nel formato PSMP.

I path remoti (host:path o user@host:path) vengono riscritti automaticamente:
  host:/path  →  corp@target@host@psmp:/path

Flag sogark (--dry-run, --force-login, -u, --key-format) devono precedere i flag scp.
Tutti gli altri flag vengono passati direttamente a scp.

Se la chiave SSH è scaduta, viene eseguita l'autenticazione automatica.`,
		Example: `  # Upload file
  sogark scp file.txt 10.1.2.3:/tmp/

  # Upload directory
  sogark scp -r ./mydir 10.1.2.3:/opt/

  # Download file
  sogark scp 10.1.2.3:/etc/hosts ./

  # Con utente target specifico
  sogark scp file.txt admin@10.1.2.3:/tmp/

  # Usa host registrato
  sogark scp file.txt myserver:/tmp/

  # Con flag scp nativi (compressione, verbose, porta)
  sogark scp -C -v -P 2222 file.txt 10.1.2.3:/tmp/

  # Dry run (mostra comando senza eseguirlo)
  sogark scp --dry-run file.txt 10.1.2.3:/tmp/

  # Forza ri-autenticazione
  sogark scp --force-login -r ./mydir 10.1.2.3:/opt/`,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Manually parse sogark-specific flags; everything else goes to scp.
			user, keyFormat, forceLogin, dryRun, scpPassArgs, err := parseScpFlags(args)
			if err != nil {
				return err
			}
			if len(scpPassArgs) == 0 {
				return fmt.Errorf("specificare source e target\nEsempio: sogark scp file.txt host:/tmp/")
			}

			cfg, err := config.Load()
			if err != nil {
				return err
			}

			targetUser := cfg.DefaultTargetUser
			if user != "" {
				targetUser = user
			}

			sogarkDir, _ := config.Dir()
			reg, _ := hosts.NewRegistry(sogarkDir)

			resolvedArgs := resolveScpArgs(scpPassArgs, reg, targetUser)

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

			keyName := cfg.SSHKeyName
			if keyFormat == "pem" {
				keyName += ".pem"
			}
			keyPath := filepath.Join(keyDir, keyName)

			scpArgs := &sshpkg.ScpArgs{
				Username:   cfg.Username,
				TargetUser: targetUser,
				ProxyHost:  cfg.ProxyHost,
				KeyPath:    keyPath,
				ScpArgs:    resolvedArgs,
			}

			fmt.Printf("> %s\n", scpArgs.CommandString())

			if dryRun {
				return nil
			}

			return scpArgs.Run()
		},
	}

	return cmd
}

// parseScpFlags separates sogark-specific flags from scp passthrough args.
// Sogark flags: --dry-run, --force-login, -u/--user <val>, --key-format <val>, -h/--help.
// Everything else is collected into passArgs for scp.
func parseScpFlags(args []string) (user, keyFormat string, forceLogin, dryRun bool, passArgs []string, err error) {
	keyFormat = "openssh"
	i := 0
	for i < len(args) {
		a := args[i]
		switch {
		case a == "--verbose":
			os.Setenv("SOGARK_DEBUG", "1")
		case a == "--dry-run":
			dryRun = true
		case a == "--force-login":
			forceLogin = true
		case a == "-u" || a == "--user":
			i++
			if i >= len(args) {
				err = fmt.Errorf("flag %s richiede un valore", a)
				return
			}
			user = args[i]
		case strings.HasPrefix(a, "--user="):
			user = strings.TrimPrefix(a, "--user=")
		case a == "--key-format":
			i++
			if i >= len(args) {
				err = fmt.Errorf("flag %s richiede un valore", a)
				return
			}
			keyFormat = args[i]
		case strings.HasPrefix(a, "--key-format="):
			keyFormat = strings.TrimPrefix(a, "--key-format=")
		case a == "-h" || a == "--help":
			err = fmt.Errorf("help")
			return
		case a == "--":
			// explicit separator: everything after goes to scp
			passArgs = append(passArgs, args[i+1:]...)
			i = len(args)
		default:
			passArgs = append(passArgs, a)
		}
		i++
	}
	return
}

// resolveScpArgs resolves host names from the registry in remote path arguments.
// For example, "myserver:/path" becomes "10.1.2.3:/path" if myserver is registered.
func resolveScpArgs(args []string, reg *hosts.Registry, defaultTargetUser string) []string {
	if reg == nil {
		return args
	}
	resolved := make([]string, len(args))
	for i, arg := range args {
		if strings.HasPrefix(arg, "-") {
			resolved[i] = arg
			continue
		}
		host, path, ok := sshpkg.ParseRemotePath(arg)
		if !ok {
			resolved[i] = arg
			continue
		}
		// Strip user@ prefix for registry lookup
		lookupHost := host
		userPrefix := ""
		if idx := strings.Index(host, "@"); idx >= 0 {
			userPrefix = host[:idx+1]
			lookupHost = host[idx+1:]
		}
		if h, found := reg.Get(lookupHost); found {
			if userPrefix == "" && h.User != "" {
				userPrefix = h.User + "@"
			}
			resolved[i] = userPrefix + h.Address + ":" + path
		} else {
			resolved[i] = arg
		}
	}
	return resolved
}
