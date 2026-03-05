package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/sogei/cyberark-cli/internal/config"
	"github.com/sogei/cyberark-cli/internal/keys"
	msg "github.com/sogei/cyberark-cli/internal/messages"
	"github.com/spf13/cobra"
)

func newKeysCmd() *cobra.Command {
	var (
		dir        string
		format     string
		forceLogin bool
	)

	cmd := &cobra.Command{
		Use:   "keys",
		Short: msg.KeysCmdShort,
		Long:  msg.KeysCmdLong,
		Example: `  sogark keys
  sogark keys --dir /tmp/deploy --format pem
  sogark keys --dir ~/.ssh --format openssh`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			keyDir := dir
			if keyDir == "" {
				keyDir, err = cfg.ResolveKeyDir()
				if err != nil {
					return err
				}
			}

			formats := cfg.KeyFormats
			if format != "" {
				formats = splitCSV(format)
				cfg.KeyFormats = formats
			}
			_ = formats

			// Check key validity
			valid, remaining, _ := keys.IsValid(keyDir, cfg.SSHKeyName, cfg.KeyTTLHours)
			if valid && !forceLogin {
				fmt.Printf(msg.KeyValidFull,
					int(remaining.Hours()), int(remaining.Minutes())%60)
			} else {
				if err := doLogin(cfg); err != nil {
					return err
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&dir, "dir", "d", "", msg.KeysFlagDir)
	cmd.Flags().StringVarP(&format, "format", "f", "", msg.KeysFlagFormat)
	cmd.Flags().BoolVar(&forceLogin, "force-login", false, msg.KeysFlagForceLogin)

	cmd.AddCommand(newKeysCleanCmd())

	return cmd
}

func newKeysCleanCmd() *cobra.Command {
	var (
		dir string
		yes bool
	)

	cmd := &cobra.Command{
		Use:   "clean",
		Short: msg.KeysCleanShort,
		Example: `  sogark keys clean
  sogark keys clean --dir /tmp/deploy
  sogark keys clean --yes`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			keyDir := dir
			if keyDir == "" {
				keyDir, err = cfg.ResolveKeyDir()
				if err != nil {
					return err
				}
			}

			if !yes {
				fmt.Printf(msg.KeysCleanPrompt, keyDir)
				reader := bufio.NewReader(os.Stdin)
				answer, _ := reader.ReadString('\n')
				answer = strings.TrimSpace(strings.ToLower(answer))
				if answer != "y" && answer != "yes" && answer != "s" && answer != "si" {
					fmt.Println(msg.KeysCleanCancelled)
					return nil
				}
			}

			removed, err := keys.Clean(keyDir, cfg.SSHKeyName)
			if err != nil {
				return err
			}

			if len(removed) == 0 {
				fmt.Println(msg.KeysCleanNoFiles)
			} else {
				fmt.Printf(msg.KeysCleanRemoved, strings.Join(removed, ", "))
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&dir, "dir", "d", "", msg.KeysCleanFlagDir)
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, msg.KeysCleanFlagYes)

	return cmd
}
