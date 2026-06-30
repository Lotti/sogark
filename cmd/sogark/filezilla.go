package main

import (
	"fmt"
	"os"

	"github.com/Lotti/sogark/internal/config"
	"github.com/Lotti/sogark/internal/keys"
	msg "github.com/Lotti/sogark/internal/messages"
	sshpkg "github.com/Lotti/sogark/internal/ssh"
	"github.com/spf13/cobra"
)

func newFileZillaCmd() *cobra.Command {
	var (
		tag           string
		anyTag        string
		filezillaPath string
	)

	cmd := &cobra.Command{
		Use:   "filezilla [host...]",
		Short: msg.FileZillaShort,
		Long:  msg.FileZillaLong,
		Example: `  sogark filezilla 10.1.2.3
  sogark filezilla myserver
  sogark filezilla --tag production
  sogark filezilla --any-tag web,db
  sogark filezilla --filezilla-path "/opt/filezilla/bin/filezilla" --tag prod`,
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

			keyPath := resolveFileZillaKeyPath(keyDir, cfg.SSHKeyName)

			fzExe, err := resolveFileZillaPath(filezillaPath, cfg)
			if err != nil {
				return err
			}

			return sshpkg.RunFileZilla(targets, cfg.Username, cfg.ProxyHost, keyPath, fzExe)
		},
	}

	cmd.Flags().StringVar(&tag, "tag", "", msg.FileZillaFlagTag)
	cmd.Flags().StringVar(&anyTag, "any-tag", "", msg.FileZillaFlagAnyTag)
	cmd.Flags().StringVar(&filezillaPath, "filezilla-path", "", msg.FileZillaFlagPath)

	return cmd
}

// resolveFileZillaPath resolves the FileZilla binary: flag → config → auto-detect.
func resolveFileZillaPath(flagPath string, cfg *config.Config) (string, error) {
	if flagPath != "" {
		return flagPath, nil
	}
	if cfg.FileZillaPath != "" {
		return cfg.FileZillaPath, nil
	}
	if p := sshpkg.FindFileZilla(); p != "" {
		return p, nil
	}
	return "", fmt.Errorf(msg.FileZillaNotFound + "\n" + msg.FileZillaNotFoundHint)
}

func resolveFileZillaKeyPath(keyDir, baseName string) string {
	openssh, ppk, pem := keyFilePaths(keyDir, baseName)

	// Prefer the same OpenSSH key that works with sogark ssh. Some FileZilla
	// Windows setups fall back to keyboard-interactive auth when fed only the
	// converted PPK, which causes unexpected PSMP password challenges.
	for _, candidate := range []string{openssh, ppk, pem} {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	return openssh
}
