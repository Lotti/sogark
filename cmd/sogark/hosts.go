package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/sogei/cyberark-cli/internal/config"
	"github.com/sogei/cyberark-cli/internal/hosts"
	msg "github.com/sogei/cyberark-cli/internal/messages"
	"github.com/spf13/cobra"
)

func newHostsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hosts",
		Short: msg.HostsShort,
	}

	cmd.AddCommand(
		newHostsAddCmd(),
		newHostsListCmd(),
		newHostsRemoveCmd(),
		newHostsTagCmd(),
		newHostsImportMobaCmd(),
		newHostsSearchCmd(),
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
		Short: msg.HostsAddShort,
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
				hostUser = cfg.DefaultSSHUser
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
				fmt.Printf(msg.HostsAddSSHConfigErr, err)
			}

			// PuTTY session (Windows only)
			if putty {
				_, ppkName, _ := keyFilePaths(keyDir, cfg.SSHKeyName)
				if err := hosts.UpdatePuTTYSession(h, cfg.Username, cfg.ProxyHost, ppkName); err != nil {
					fmt.Printf(msg.HostsAddPuTTYErr, err)
				} else {
					fmt.Printf(msg.HostsAddPuTTYSuccess, name)
				}
			}

			fmt.Printf(msg.HostsAdded, name, address)
			if len(tagList) > 0 {
				fmt.Printf("  Tag: %s\n", strings.Join(tagList, ", "))
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&user, "user", "u", "", msg.HostsAddFlagUser)
	cmd.Flags().StringVar(&tags, "tags", "", msg.HostsAddFlagTags)
	cmd.Flags().BoolVar(&putty, "putty", false, msg.HostsAddFlagPutty)

	return cmd
}

func newHostsListCmd() *cobra.Command {
	var (
		tag    string
		anyTag string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: msg.HostsListShort,
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
				fmt.Println(msg.HostsListNoneFound)
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
			fmt.Printf(msg.HostsListCount, len(hostList))

			return nil
		},
	}

	cmd.Flags().StringVar(&tag, "tag", "", msg.HostsListFlagTag)
	cmd.Flags().StringVar(&anyTag, "any-tag", "", msg.HostsListFlagAnyTag)

	return cmd
}

func newHostsRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: msg.HostsRemoveShort,
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

			fmt.Printf(msg.HostsRemoved, name)
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
		Short: msg.HostsTagShort,
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

	cmd.Flags().StringVar(&addTags, "add", "", msg.HostsTagFlagAdd)
	cmd.Flags().StringVar(&removeTags, "remove", "", msg.HostsTagFlagRemove)

	return cmd
}

func keyFilePaths(keyDir, baseName string) (openssh, ppk, pem string) {
	openssh = filepath.Join(keyDir, baseName)
	ppk = filepath.Join(keyDir, baseName+".ppk")
	pem = filepath.Join(keyDir, baseName+".pem")
	return
}

func newHostsImportMobaCmd() *cobra.Command {
	var (
		extraTag string
		dryRun   bool
		noUser   bool
	)

	cmd := &cobra.Command{
		Use:   "import-moba <file.mxtsessions>",
		Short: msg.HostsImportMobaShort,
		Long:  msg.HostsImportMobaLong,
		Example: `  sogark hosts import-moba sessions.mxtsessions
  sogark hosts import-moba --tag production sessions.mxtsessions
  sogark hosts import-moba --no-user sessions.mxtsessions
  sogark hosts import-moba --dry-run sessions.mxtsessions`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sessions, err := hosts.ParseMobaFile(args[0])
			if err != nil {
				return err
			}

			if len(sessions) == 0 {
				fmt.Println(msg.HostsImportMobaNoSessions)
				return nil
			}

			if dryRun {
				fmt.Printf(msg.HostsImportMobaPreview, len(sessions))
				for _, s := range sessions {
					tags := s.Tags
					if extraTag != "" {
						tags = append(tags, extraTag)
					}
					tagStr := ""
					if len(tags) > 0 {
						tagStr = " [" + strings.Join(tags, ", ") + "]"
					}
					user := s.User
					if noUser {
						user = msg.HostsImportMobaUserIgnored
					} else if user == "" {
						user = msg.HostsImportMobaUserDefault
					}
					fmt.Printf("    %-20s %s (user: %s)%s\n", s.Name, s.Address, user, tagStr)
				}
				return nil
			}

			reg, _, err := loadRegistry()
			if err != nil {
				return err
			}

			imported := 0
			for _, s := range sessions {
				tags := s.Tags
				if extraTag != "" {
					tags = append(tags, extraTag)
				}
				user := s.User
				if noUser {
					user = ""
				}
				reg.Add(s.Name, s.Address, user, tags)
				imported++
			}

			if err := reg.Save(); err != nil {
				return fmt.Errorf(msg.HostsImportMobaErrSave, err)
			}

			fmt.Printf(msg.HostsImportMobaSuccess, imported)
			return nil
		},
	}

	cmd.Flags().StringVar(&extraTag, "tag", "", msg.HostsImportMobaFlagTag)
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, msg.HostsImportMobaFlagDryRun)
	cmd.Flags().BoolVar(&noUser, "no-user", false, msg.HostsImportMobaFlagNoUser)

	return cmd
}

func newHostsSearchCmd() *cobra.Command {
	var (
		namePattern string
		ipPattern   string
		tagFilter   string
		addTags     string
		removeTags  string
	)

	cmd := &cobra.Command{
		Use:   "search [pattern]",
		Short: msg.HostsSearchShort,
		Long:  msg.HostsSearchLong,
		Example: `  sogark hosts search                        # tutti gli host
  sogark hosts search "web*"                 # nomi che iniziano con "web"
  sogark hosts search --name "*db*"
  sogark hosts search --ip "10.50.1.*"
  sogark hosts search --tag prod
  sogark hosts search --name "web*" --tag prod --add-tag reviewed
  sogark hosts search --ip "10.0.*" --remove-tag old`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// positional arg is a shorthand for --name
			if len(args) == 1 && namePattern == "" {
				namePattern = args[0]
			}

			reg, _, err := loadRegistry()
			if err != nil {
				return err
			}

			var tagList []string
			if tagFilter != "" {
				tagList = splitCSV(tagFilter)
			}

			results := reg.Search(namePattern, ipPattern, tagList)

			if len(results) == 0 {
				fmt.Println(msg.HostsSearchNoneFound)
				return nil
			}

			// If editing tags, apply changes and save
			doEdit := addTags != "" || removeTags != ""
			if doEdit {
				addList := splitCSV(addTags)
				removeList := splitCSV(removeTags)
				for _, h := range results {
					if len(addList) > 0 {
						_ = reg.AddTags(h.Name, addList)
					}
					if len(removeList) > 0 {
						_ = reg.RemoveTags(h.Name, removeList)
					}
				}
				if err := reg.Save(); err != nil {
					return fmt.Errorf(msg.HostsSearchErrSave, err)
				}
				fmt.Printf(msg.HostsSearchTagsUpdated, len(results))
				// Re-read updated hosts for display
				results = reg.Search(namePattern, ipPattern, tagList)
			}

			fmt.Printf("%-20s %-20s %-15s %s\n", "NAME", "ADDRESS", "USER", "TAG")
			fmt.Println(strings.Repeat("─", 70))
			for _, h := range results {
				user := h.User
				if user == "" {
					user = "-"
				}
				tags := strings.Join(h.Tags, ", ")
				if tags == "" {
					tags = "-"
				}
				fmt.Printf("%-20s %-20s %-15s %s\n", h.Name, h.Address, user, tags)
			}
			fmt.Printf(msg.HostsSearchCount, len(results))
			return nil
		},
	}

	cmd.Flags().StringVar(&namePattern, "name", "", msg.HostsSearchFlagName)
	cmd.Flags().StringVar(&ipPattern, "ip", "", msg.HostsSearchFlagIP)
	cmd.Flags().StringVar(&tagFilter, "tag", "", msg.HostsSearchFlagTag)
	cmd.Flags().StringVar(&addTags, "add-tag", "", msg.HostsSearchFlagAddTag)
	cmd.Flags().StringVar(&removeTags, "remove-tag", "", msg.HostsSearchFlagRemoveTag)

	return cmd
}
