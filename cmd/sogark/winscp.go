package main

import (
	"fmt"
	"path/filepath"

	"github.com/Lotti/sogark/internal/config"
	"github.com/Lotti/sogark/internal/keys"
	msg "github.com/Lotti/sogark/internal/messages"
	sshpkg "github.com/Lotti/sogark/internal/ssh"
	"github.com/spf13/cobra"
)

func newWinSCPCmd() *cobra.Command {
	var (
		tag        string
		anyTag     string
		winscpPath string
	)

	cmd := &cobra.Command{
		Use:   "winscp [host...]",
		Short: msg.WinSCPShort,
		Long:  msg.WinSCPLong,
		Example: `  sogark winscp 10.1.2.3
  sogark winscp myserver
  sogark winscp --tag production
  sogark winscp --any-tag web,db
  sogark winscp --winscp-path "C:\WinSCP\WinSCP.exe" --tag prod`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			targets, err := resolveTargets(cfg, args, tag, anyTag)
			if err != nil {
				return err
			}

			keyDir, err := cfg.ResolveKeyDir()
			if err != nil {
				return err
			}

			// Ensure valid key
			valid, _, _ := keys.IsValid(keyDir, cfg.SSHKeyName, cfg.KeyTTLHours)
			if !valid {
				fmt.Println(msg.KeyExpired)
				if err := doLogin(cfg); err != nil {
					return err
				}
			}

			// Prefer .ppk for WinSCP
			keyPath := filepath.Join(keyDir, cfg.SSHKeyName+".ppk")

			winscpExe, err := resolveWinSCPPath(winscpPath, cfg)
			if err != nil {
				return err
			}

			return sshpkg.RunWinSCP(targets, cfg.Username, cfg.ProxyHost, keyPath, winscpExe)
		},
	}

	cmd.Flags().StringVar(&tag, "tag", "", msg.WinSCPFlagTag)
	cmd.Flags().StringVar(&anyTag, "any-tag", "", msg.WinSCPFlagAnyTag)
	cmd.Flags().StringVar(&winscpPath, "winscp-path", "", msg.WinSCPFlagPath)

	return cmd
}

// resolveWinSCPPath resolves the WinSCP executable: flag → config → auto-detect.
func resolveWinSCPPath(flagPath string, cfg *config.Config) (string, error) {
	if flagPath != "" {
		return flagPath, nil
	}
	if cfg.WinSCPPath != "" {
		return cfg.WinSCPPath, nil
	}
	if p := sshpkg.FindWinSCP(); p != "" {
		return p, nil
	}
	return "", fmt.Errorf(msg.WinSCPNotFound)
}
