package main

import (
	"github.com/Lotti/sogark/internal/config"
	msg "github.com/Lotti/sogark/internal/messages"
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

			return doLoginWithFormats(cfg, formats)
		},
	}

	cmd.Flags().StringVarP(&user, "user", "u", "", msg.LoginFlagUser)
	cmd.Flags().StringVarP(&format, "format", "f", "", msg.LoginFlagFormat)

	return cmd
}
