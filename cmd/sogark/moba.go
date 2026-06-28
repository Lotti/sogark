package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Lotti/sogark/internal/config"
	"github.com/Lotti/sogark/internal/keys"
	msg "github.com/Lotti/sogark/internal/messages"
	sshpkg "github.com/Lotti/sogark/internal/ssh"
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
		Short: msg.MobaShort,
		Long:  msg.MobaLong,
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
				fmt.Println(msg.KeyExpired)
				if err := doLogin(cfg); err != nil {
					return err
				}
			}

			// Resolve MobaXterm path: flag > config > auto-detect > prompt
			mobaExe := resolveMobaPath(mobaPath, cfg)

			return sshpkg.RunMoba(targets, cfg.Username, cfg.ProxyHost, keyPath, mobaExe, cfg.MobaMaxSessions)
		},
	}

	cmd.Flags().StringVar(&tag, "tag", "", msg.MobaFlagTag)
	cmd.Flags().StringVar(&anyTag, "any-tag", "", msg.MobaFlagAnyTag)
	cmd.Flags().StringVar(&mobaPath, "moba-path", "", msg.MobaFlagPath)

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
	fmt.Println(msg.MobaNotFound)
	fmt.Print(msg.MobaEnterPath)
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input == "" {
		return ""
	}

	// Validate the path exists
	if _, err := os.Stat(input); err != nil {
		fmt.Fprintf(os.Stderr, msg.MobaFileNotFound, input)
		return ""
	}

	// Save to config for future runs
	cfg.MobaPath = input
	if err := cfg.Save(); err != nil {
		fmt.Fprintf(os.Stderr, msg.MobaErrSavingConfig, err)
	} else {
		fmt.Printf(msg.MobaPathSaved, input)
	}

	return input
}
