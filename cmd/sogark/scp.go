package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/sogei/cyberark-cli/internal/config"
	"github.com/sogei/cyberark-cli/internal/hosts"
	"github.com/sogei/cyberark-cli/internal/keys"
	sshpkg "github.com/sogei/cyberark-cli/internal/ssh"
	"github.com/spf13/cobra"
)

func newScpCmd() *cobra.Command {
	var (
		user       string
		keyFormat  string
		forceLogin bool
		dryRun     bool
	)

	cmd := &cobra.Command{
		Use:   "scp [flags] -- [scp-args...] source... target",
		Short: "Trasferimento file via SCP attraverso PSMP",
		Long: `Wrapper trasparente per scp: tutti i flag nativi di scp passano dopo --.
sogark si occupa di iniettare la chiave SSH (-i) e tradurre i path remoti nel formato PSMP.

I path remoti (host:path o user@host:path) vengono riscritti automaticamente:
  host:/path  →  corp@target@host@psmp:/path

Se la chiave SSH è scaduta, viene eseguita l'autenticazione automatica.`,
		Example: `  # Upload file
  sogark scp -- file.txt 10.1.2.3:/tmp/

  # Upload directory
  sogark scp -- -r ./mydir 10.1.2.3:/opt/

  # Download file
  sogark scp -- 10.1.2.3:/etc/hosts ./

  # Con utente target specifico
  sogark scp -- file.txt admin@10.1.2.3:/tmp/

  # Usa host registrato
  sogark scp -- file.txt myserver:/tmp/

  # Con flag scp nativi (compressione, verbose, porta)
  sogark scp -- -C -v -P 2222 file.txt 10.1.2.3:/tmp/

  # Dry run (mostra comando senza eseguirlo)
  sogark scp --dry-run -- file.txt 10.1.2.3:/tmp/

  # Forza ri-autenticazione
  sogark scp --force-login -- file.txt 10.1.2.3:/tmp/`,
		DisableFlagParsing: false,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("specificare argomenti scp dopo --\nEsempio: sogark scp -- file.txt host:/path")
			}

			cfg, err := config.Load()
			if err != nil {
				return err
			}

			// Resolve host names from registry in remote path args
			targetUser := cfg.DefaultTargetUser
			if user != "" {
				targetUser = user
			}

			sogarkDir, _ := config.Dir()
			reg, _ := hosts.NewRegistry(sogarkDir)

			resolvedArgs := resolveScpArgs(args, reg, targetUser)

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

	cmd.Flags().StringVarP(&user, "user", "u", "", "utente target sulla macchina remota (override)")
	cmd.Flags().StringVar(&keyFormat, "key-format", "openssh", "formato chiave da usare: openssh, pem")
	cmd.Flags().BoolVar(&forceLogin, "force-login", false, "forza ri-autenticazione anche se la chiave è valida")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "mostra il comando scp senza eseguirlo")

	return cmd
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
