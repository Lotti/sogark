package main

import (
	"fmt"
	"path/filepath"

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
		Short: "Esecuzione parallela di un comando su più host",
		Long:  `Esegue un comando su più host in parallelo e raccoglie l'output con prefisso [hostname].`,
		Example: `  sogark exec --tag webservers "uptime"
  sogark exec web1 web2 "systemctl status nginx"
  sogark exec --any-tag web,db "cat /etc/hostname"`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			// Last arg is the command; preceding args are host names (when no tag flag)
			var hostArgs []string
			var command string

			if tag != "" || anyTag != "" {
				// All args are the command
				command = args[0]
			} else {
				if len(args) < 2 {
					return fmt.Errorf("specifica almeno un host e un comando, oppure usa --tag")
				}
				hostArgs = args[:len(args)-1]
				command = args[len(args)-1]
			}

			targets, err := resolveTargets(cfg, hostArgs, tag, anyTag)
			if err != nil {
				return err
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
