package main

import (
	"fmt"
	"path/filepath"
	"runtime"

	"github.com/sogei/cyberark-cli/internal/config"
	"github.com/sogei/cyberark-cli/internal/keys"
	msg "github.com/sogei/cyberark-cli/internal/messages"
	sshpkg "github.com/sogei/cyberark-cli/internal/ssh"
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

			// FileZilla works with OpenSSH keys on macOS/Linux, PPK on Windows
			keyExt := ""
			if runtime.GOOS == "windows" {
				keyExt = ".ppk"
			}
			keyPath := filepath.Join(keyDir, cfg.SSHKeyName+keyExt)

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