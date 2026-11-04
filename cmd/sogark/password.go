package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/Lotti/sogark/internal/auth"
	"github.com/Lotti/sogark/internal/clipboard"
	"github.com/Lotti/sogark/internal/config"
	"github.com/spf13/cobra"
)

func newPasswordCmd() *cobra.Command {
	var reason, ticketID, user string
	cmd := &cobra.Command{
		Use:   "password <machine>",
		Short: "Retrieve a CyberArk account password to the system clipboard",
		Long:  "Authenticates with CyberArk Identity, searches PVWA accounts for the machine and target username, then copies the matching password to the system clipboard. The password is never printed to the terminal.",
		Args:  cobra.ExactArgs(1),
		Example: `  sogark password 123_45 --reason "maintenance"
  sogark password app01 --user root --reason "maintenance"
  sogark password app01.example.org --reason "incident response" --ticket INC001234`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			profile, err := cfg.SelectedAuthProfile()
			if err != nil {
				return err
			}
			client, err := auth.NewNativeClient(profile)
			if err != nil {
				return err
			}
			prompter, err := auth.NewTerminalPrompter(os.Stdin, os.Stdout)
			if err != nil {
				return err
			}
			authCtx, cancel := context.WithTimeout(signalCtx, time.Duration(cfg.AuthTimeoutMinutes)*time.Minute)
			defer cancel()
			fmt.Printf("[*] Authentication profile: %s (%s)\n", cfg.AuthProfile, profile.AuthType)
			if err := client.AuthenticateIdentity(authCtx, cfg.Username, prompter); err != nil {
				return err
			}
			fmt.Println("[+] Identity QR/push authentication complete")
			if err := client.EstablishPVWA(authCtx); err != nil {
				return err
			}
			targetUser := user
			if targetUser == "" {
				targetUser = cfg.DefaultSSHUser
			}
			accountID, err := client.FindAccountID(authCtx, args[0], targetUser)
			if err != nil {
				return err
			}
			password, err := client.RetrieveAccountPassword(authCtx, accountID, reason, ticketID)
			if err != nil {
				return err
			}
			if err := clipboard.Write(authCtx, password); err != nil {
				return err
			}
			fmt.Println("[+] Account password copied to clipboard")
			return nil
		},
	}
	cmd.Flags().StringVarP(&user, "user", "u", "", "target account username (defaults to default_ssh_user in config)")
	cmd.Flags().StringVar(&reason, "reason", "", "reason for retrieving the account password (if required by policy)")
	cmd.Flags().StringVar(&ticketID, "ticket", "", "ticket ID for the retrieval request (if required by policy)")
	return cmd
}
