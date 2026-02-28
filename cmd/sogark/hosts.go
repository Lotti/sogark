package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/sogei/cyberark-cli/internal/config"
	"github.com/sogei/cyberark-cli/internal/hosts"
	"github.com/spf13/cobra"
)

func newHostsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hosts",
		Short: "Gestione registro macchine con tag",
	}

	cmd.AddCommand(
		newHostsAddCmd(),
		newHostsListCmd(),
		newHostsRemoveCmd(),
		newHostsTagCmd(),
	)

	return cmd
}

func loadRegistry() (*hosts.Registry, *config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, err
	}
	sogarkDir, err := config.Dir()
	if err != nil {
		return nil, nil, err
	}
	reg, err := hosts.NewRegistry(sogarkDir)
	if err != nil {
		return nil, nil, err
	}
	return reg, cfg, nil
}

func newHostsAddCmd() *cobra.Command {
	var (
		user  string
		tags  string
		putty bool
	)

	cmd := &cobra.Command{
		Use:   "add <name> <address>",
		Short: "Registra un host con tag opzionali",
		Example: `  sogark hosts add web1 10.1.2.1 --tags webservers,production
  sogark hosts add db1 10.1.2.3 --user admin --tags databases
  sogark hosts add web1 10.1.2.1 --putty`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			reg, cfg, err := loadRegistry()
			if err != nil {
				return err
			}

			name, address := args[0], args[1]
			hostUser := user
			if hostUser == "" {
				hostUser = cfg.DefaultTargetUser
			}

			var tagList []string
			if tags != "" {
				tagList = splitCSV(tags)
			}

			reg.Add(name, address, hostUser, tagList)
			if err := reg.Save(); err != nil {
				return err
			}

			// Update SSH config
			h, _ := reg.Get(name)
			keyDir, _ := cfg.ResolveKeyDir()
			keyPath := filepath.Join(keyDir, cfg.SSHKeyName)
			if err := hosts.UpdateSSHConfig(h, cfg.Username, cfg.ProxyHost, keyPath); err != nil {
				fmt.Printf("[!] Aggiornamento ~/.ssh/config fallito: %v\n", err)
			}

			// PuTTY session (Windows only)
			if putty {
				_, ppkName, _ := keyFilePaths(keyDir, cfg.SSHKeyName)
				if err := hosts.UpdatePuTTYSession(h, cfg.Username, cfg.ProxyHost, ppkName); err != nil {
					fmt.Printf("[!] Sessione PuTTY: %v\n", err)
				} else {
					fmt.Printf("[+] Sessione PuTTY creata: %s\n", name)
				}
			}

			fmt.Printf("[+] Host aggiunto: %s (%s)\n", name, address)
			if len(tagList) > 0 {
				fmt.Printf("  Tag: %s\n", strings.Join(tagList, ", "))
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&user, "user", "u", "", "utente target (default: dalla config)")
	cmd.Flags().StringVar(&tags, "tags", "", "tag separati da virgola")
	cmd.Flags().BoolVar(&putty, "putty", false, "crea anche sessione PuTTY (solo Windows)")

	return cmd
}

func newHostsListCmd() *cobra.Command {
	var (
		tag    string
		anyTag string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "Lista host registrati (filtro per tag)",
		Example: `  sogark hosts list
  sogark hosts list --tag production
  sogark hosts list --tag webservers,rome
  sogark hosts list --any-tag web,db`,
		RunE: func(cmd *cobra.Command, args []string) error {
			reg, _, err := loadRegistry()
			if err != nil {
				return err
			}

			var hostList []*hosts.Host
			switch {
			case tag != "":
				hostList = reg.ByTagsAND(splitCSV(tag))
			case anyTag != "":
				hostList = reg.ByTagsOR(splitCSV(anyTag))
			default:
				hostList = reg.All()
			}

			if len(hostList) == 0 {
				fmt.Println("Nessun host trovato.")
				return nil
			}

			for _, h := range hostList {
				tagsStr := ""
				if len(h.Tags) > 0 {
					tagsStr = " [" + strings.Join(h.Tags, ", ") + "]"
				}
				userStr := ""
				if h.User != "" {
					userStr = h.User + "@"
				}
				fmt.Printf("  %-15s %s%s%s\n", h.Name, userStr, h.Address, tagsStr)
			}
			fmt.Printf("\n%d host\n", len(hostList))

			return nil
		},
	}

	cmd.Flags().StringVar(&tag, "tag", "", "filtra per tag (AND: tutti i tag devono corrispondere)")
	cmd.Flags().StringVar(&anyTag, "any-tag", "", "filtra per tag (OR: almeno un tag)")

	return cmd
}

func newHostsRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Rimuovi un host dal registro",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			reg, _, err := loadRegistry()
			if err != nil {
				return err
			}

			name := args[0]
			if err := reg.Remove(name); err != nil {
				return err
			}
			if err := reg.Save(); err != nil {
				return err
			}

			// Clean up SSH config
			_ = hosts.RemoveSSHConfig(name)
			// Clean up PuTTY session (best effort)
			_ = hosts.RemovePuTTYSession(name)

			fmt.Printf("[+] Host rimosso: %s\n", name)
			return nil
		},
	}
}

func newHostsTagCmd() *cobra.Command {
	var (
		addTags    string
		removeTags string
	)

	cmd := &cobra.Command{
		Use:   "tag <name>",
		Short: "Gestisci i tag di un host",
		Example: `  sogark hosts tag web1 --add production,rome
  sogark hosts tag web1 --remove staging`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			reg, _, err := loadRegistry()
			if err != nil {
				return err
			}

			name := args[0]

			if addTags != "" {
				if err := reg.AddTags(name, splitCSV(addTags)); err != nil {
					return err
				}
			}
			if removeTags != "" {
				if err := reg.RemoveTags(name, splitCSV(removeTags)); err != nil {
					return err
				}
			}

			if err := reg.Save(); err != nil {
				return err
			}

			h, _ := reg.Get(name)
			fmt.Printf("[+] %s tag: %s\n", name, strings.Join(h.Tags, ", "))
			return nil
		},
	}

	cmd.Flags().StringVar(&addTags, "add", "", "tag da aggiungere")
	cmd.Flags().StringVar(&removeTags, "remove", "", "tag da rimuovere")

	return cmd
}

func keyFilePaths(keyDir, baseName string) (openssh, ppk, pem string) {
	openssh = filepath.Join(keyDir, baseName)
	ppk = filepath.Join(keyDir, baseName+".ppk")
	pem = filepath.Join(keyDir, baseName+".pem")
	return
}
