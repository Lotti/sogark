package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/sogei/cyberark-cli/internal/config"
	"github.com/sogei/cyberark-cli/internal/keys"
	sshpkg "github.com/sogei/cyberark-cli/internal/ssh"
	"github.com/spf13/cobra"
)

func newExecCmd() *cobra.Command {
	var (
		tag    string
		anyTag string
	)

	cmd := &cobra.Command{
		Use:   "exec [host...] <command>",
		Short: "Esecuzione di un comando su più host via tmux/WezTerm",
		Long: `Apre sessioni SSH interattive, sincronizza i pane e digita
il comando automaticamente. Resta attaccato per comandi successivi.
Richiede tmux o WezTerm (CyberArk PSMP richiede sessioni interattive).`,
		Example: `  sogark exec --tag webservers "uptime"
  sogark exec #webservers "uptime"
  sogark exec oper1@#web#prod "systemctl status nginx"
  sogark exec web1 web2 "systemctl status nginx"
  sogark exec --any-tag web,db "cat /etc/hostname"`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			var hostArgs []string
			var command string
			tagOverride := tag
			var userOverride string

			if tag != "" || anyTag != "" {
				command = args[0]
			} else if len(args) >= 2 {
				// Check if first arg is a #tag selector
				if u, tags, ok := parseTagArg(args[0]); ok {
					tagOverride = strings.Join(tags, ",")
					userOverride = u
					command = args[1]
				} else {
					hostArgs = args[:len(args)-1]
					command = args[len(args)-1]
				}
			} else {
				return fmt.Errorf("specifica almeno un host e un comando, oppure usa --tag")
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

			return sshpkg.RunExec(targets, command, cfg.Username, cfg.ProxyHost, keyPath)
		},
	}

	cmd.Flags().StringVar(&tag, "tag", "", "filtra per tag (AND)")
	cmd.Flags().StringVar(&anyTag, "any-tag", "", "filtra per tag (OR)")

	return cmd
}
