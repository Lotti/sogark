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

func newMultiCmd() *cobra.Command {
	var (
		tag    string
		anyTag string
		noSync bool
	)

	cmd := &cobra.Command{
		Use:   "multi [host...]",
		Short: "Sessioni SSH parallele in tmux con pane sincronizzati",
		Long:  `Apre una sessione tmux con un pane per ogni host, con synchronize-panes abilitato.`,
		Example: `  sogark multi --tag production
  sogark multi web1 web2 db1
  sogark multi --any-tag web,db
  sogark multi --tag prod --no-sync`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			targets, err := resolveTargets(cfg, args, tag, anyTag)
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

			multiArgs := &sshpkg.MultiArgs{
				Hosts: targets,
				Sync:  !noSync,
			}

			return sshpkg.RunMulti(multiArgs, cfg.Username, cfg.ProxyHost, keyPath)
		},
	}

	cmd.Flags().StringVar(&tag, "tag", "", "filtra per tag (AND)")
	cmd.Flags().StringVar(&anyTag, "any-tag", "", "filtra per tag (OR)")
	cmd.Flags().BoolVar(&noSync, "no-sync", false, "non sincronizzare l'input tra i pane")

	return cmd
}

// resolveTargets resolves host names/tags to HostTarget list.
func resolveTargets(cfg *config.Config, args []string, tag, anyTag string) ([]sshpkg.HostTarget, error) {
	sogarkDir, _ := config.Dir()
	reg, err := hosts.NewRegistry(sogarkDir)
	if err != nil {
		return nil, err
	}

	var hostList []*hosts.Host
	switch {
	case tag != "":
		hostList = reg.ByTagsAND(splitCSV(tag))
	case anyTag != "":
		hostList = reg.ByTagsOR(splitCSV(anyTag))
	case len(args) > 0:
		for _, name := range args {
			if h, ok := reg.Get(name); ok {
				hostList = append(hostList, h)
			} else {
				// Treat as direct address
				hostList = append(hostList, &hosts.Host{
					Name:    name,
					Address: name,
					User:    cfg.DefaultTargetUser,
				})
			}
		}
	default:
		return nil, fmt.Errorf("specifica host o tag (--tag / --any-tag)")
	}

	if len(hostList) == 0 {
		return nil, fmt.Errorf("nessun host trovato")
	}

	targets := make([]sshpkg.HostTarget, len(hostList))
	for i, h := range hostList {
		user := h.User
		if user == "" {
			user = cfg.DefaultTargetUser
		}
		targets[i] = sshpkg.HostTarget{
			Name:       h.Name,
			Address:    h.Address,
			TargetUser: user,
		}
	}

	fmt.Printf("Host selezionati: %s\n", formatHostNames(targets))
	return targets, nil
}

func formatHostNames(targets []sshpkg.HostTarget) string {
	names := make([]string, len(targets))
	for i, t := range targets {
		names[i] = t.Name
	}
	return strings.Join(names, ", ")
}
