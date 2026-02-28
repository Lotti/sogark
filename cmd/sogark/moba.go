package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sogei/cyberark-cli/internal/config"
	"github.com/sogei/cyberark-cli/internal/keys"
	sshpkg "github.com/sogei/cyberark-cli/internal/ssh"
	"github.com/spf13/cobra"
)

func newMobaCmd() *cobra.Command {
	var (
		tag      string
		anyTag   string
		mobaPath string
	)

	cmd := &cobra.Command{
		Use:   "moba [host...]",
		Short: "Apri sessioni SSH in MobaXterm",
		Long: `Apre MobaXterm con un tab SSH per ogni host selezionato.
Dopo l'apertura, attiva MultiExec per inviare comandi a tutti i tab.`,
		Example: `  sogark moba #production
  sogark moba oper1@#web#prod
  sogark moba --tag webservers
  sogark moba web1 web2 db1
  sogark moba --moba-path "C:\Tools\MobaXterm.exe" #production`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			var hostArgs []string
			tagOverride := tag
			var userOverride string

			if tag == "" && anyTag == "" && len(args) > 0 {
				// Check if first arg is a #tag selector
				if u, tags, ok := parseTagArg(args[0]); ok {
					tagOverride = strings.Join(tags, ",")
					userOverride = u
				} else {
					hostArgs = args
				}
			}

			targets, err := resolveTargets(cfg, hostArgs, tagOverride, anyTag)
			if err != nil {
				return err
			}

			if userOverride != "" {
				for i := range targets {
					targets[i].TargetUser = userOverride
				}
			}

			keyDir, _ := cfg.ResolveKeyDir()
			keyPath := filepath.Join(keyDir, cfg.SSHKeyName)

			// Ensure valid key
			valid, _, _ := keys.IsValid(keyDir, cfg.SSHKeyName, cfg.KeyTTLHours)
			if !valid {
				fmt.Println("[!] Chiave scaduta o assente, avvio autenticazione...")
				if err := doLogin(cfg); err != nil {
					return err
				}
			}

			// Resolve MobaXterm path: flag > config > auto-detect > prompt
			mobaExe := resolveMobaPath(mobaPath, cfg)

			return sshpkg.RunMoba(targets, cfg.Username, cfg.ProxyHost, keyPath, mobaExe)
		},
	}

	cmd.Flags().StringVar(&tag, "tag", "", "filtra per tag (AND)")
	cmd.Flags().StringVar(&anyTag, "any-tag", "", "filtra per tag (OR)")
	cmd.Flags().StringVar(&mobaPath, "moba-path", "", "percorso MobaXterm.exe")

	return cmd
}

// resolveMobaPath resolves the MobaXterm executable path.
// Priority: flag > config > auto-detect > interactive prompt (saves to config).
func resolveMobaPath(flagPath string, cfg *config.Config) string {
	if flagPath != "" {
		return flagPath
	}
	if cfg.MobaPath != "" {
		return cfg.MobaPath
	}
	if found := sshpkg.FindMobaXterm(); found != "" {
		return found
	}

	// Interactive prompt
	fmt.Println("[!] MobaXterm non trovato.")
	fmt.Print("    Inserisci il percorso di MobaXterm.exe: ")
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input == "" {
		return ""
	}

	// Validate the path exists
	if _, err := os.Stat(input); err != nil {
		fmt.Fprintf(os.Stderr, "[!] File non trovato: %s\n", input)
		return ""
	}

	// Save to config for future runs
	cfg.MobaPath = input
	if err := cfg.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "[!] Errore salvataggio config: %v\n", err)
	} else {
		fmt.Printf("[+] Percorso salvato nella configurazione: %s\n", input)
	}

	return input
}
