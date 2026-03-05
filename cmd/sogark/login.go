package main

import (
	"fmt"

	"github.com/sogei/cyberark-cli/internal/auth"
	"github.com/sogei/cyberark-cli/internal/config"
	"github.com/sogei/cyberark-cli/internal/keys"
	msg "github.com/sogei/cyberark-cli/internal/messages"
	"github.com/spf13/cobra"
)

func newLoginCmd() *cobra.Command {
	var (
		user   string
		format string
	)

	cmd := &cobra.Command{
		Use:   "login",
		Short: msg.LoginShort,
		Long:  msg.LoginLong,
		Example: `  sogark login
  sogark login --user mario.rossi
  sogark login --format openssh,pem`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			if user != "" {
				cfg.Username = user
			}

			formats := cfg.KeyFormats
			if format != "" {
				formats = splitCSV(format)
			}

			samlResponse, err := auth.SAMLResponse(signalCtx, cfg.IDPURL, cfg.SAMLTimeoutMinutes)
			if err != nil {
				return err
			}

			client := auth.NewClient(cfg.PVWABaseURL)
			if err := client.Logon(samlResponse); err != nil {
				return err
			}

			fmt.Println(msg.DownloadingKeys)
			raw, err := client.FetchSSHKeys(formats)
			if err != nil {
				return err
			}

			parsed, err := keys.Parse(raw)
			if err != nil {
				return err
			}

			keyDir, err := cfg.ResolveKeyDir()
			if err != nil {
				return err
			}

			results, err := keys.Save(parsed, keyDir, cfg.SSHKeyName, formats)
			if err != nil {
				return err
			}

			if err := keys.SaveTimestamp(keyDir); err != nil {
				return err
			}

			fmt.Println(msg.KeysSaved)
			for _, r := range results {
				fmt.Printf("    %-40s (%s)\n", r.Path, r.Format)
			}
			fmt.Printf(msg.KeysExpiry, cfg.KeyTTLHours)

			return nil
		},
	}

	cmd.Flags().StringVarP(&user, "user", "u", "", msg.LoginFlagUser)
	cmd.Flags().StringVarP(&format, "format", "f", "", msg.LoginFlagFormat)

	return cmd
}
