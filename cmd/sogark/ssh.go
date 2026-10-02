package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Lotti/sogark/internal/auth"
	"github.com/Lotti/sogark/internal/config"
	"github.com/Lotti/sogark/internal/hosts"
	"github.com/Lotti/sogark/internal/keys"
	msg "github.com/Lotti/sogark/internal/messages"
	sshpkg "github.com/Lotti/sogark/internal/ssh"
	"github.com/spf13/cobra"
)

func newSSHCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ssh [sogark-flags] [user@]host [ssh-args...]",
		Short: msg.SSHShort,
		Long:  msg.SSHLong,
		Example: `  sogark ssh 10.1.2.3
  sogark ssh admin@10.1.2.3
  sogark ssh myserver
  sogark ssh 10.1.2.3 -L 8080:localhost:80
  sogark ssh 10.1.2.3 -v -o StrictHostKeyChecking=no
  sogark ssh 10.1.2.3 -D 1080
  sogark ssh --dry-run 10.1.2.3`,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			user, keyFormat, forceLogin, dryRun, host, sshExtraArgs, err := parseSSHFlags(args)
			if err != nil {
				if err.Error() == "help" {
					return cmd.Help()
				}
				return err
			}
			if host == "" {
				return fmt.Errorf(msg.SSHErrNoHost)
			}

			cfg, err := config.Load()
			if err != nil {
				return err
			}

			// Resolve from hosts registry
			targetUser, resolvedHost := sshpkg.ParseTarget(host, cfg.DefaultSSHUser)
			if user != "" {
				targetUser = user
			}

			sogarkDir, _ := config.Dir()
			reg, _ := hosts.NewRegistry(sogarkDir)
			if reg != nil {
				if h, ok := reg.Get(resolvedHost); ok {
					resolvedHost = h.Address
					if h.User != "" && user == "" && !strings.Contains(host, "@") {
						targetUser = h.User
					}
				}
			}

			keyDir, err := cfg.ResolveKeyDir()
			if err != nil {
				return err
			}

			// Check key validity
			valid, remaining, _ := keys.IsValid(keyDir, cfg.SSHKeyName, cfg.KeyTTLHours)
			if valid && !forceLogin {
				fmt.Printf(msg.KeyValid, formatDuration(remaining))
			} else {
				if !valid {
					fmt.Println(msg.KeyExpired)
				}
				if err := doLogin(cfg); err != nil {
					return err
				}
			}

			// Determine key path
			keyName := cfg.SSHKeyName
			if keyFormat == "pem" {
				keyName += ".pem"
			}
			keyPath := filepath.Join(keyDir, keyName)

			connectArgs := &sshpkg.ConnectArgs{
				Username:   cfg.Username,
				TargetUser: targetUser,
				Host:       resolvedHost,
				ProxyHost:  cfg.ProxyHost,
				KeyPath:    keyPath,
				ExtraArgs:  sshExtraArgs,
			}

			fmt.Printf("> %s\n", connectArgs.CommandString())

			if dryRun {
				return nil
			}

			return connectArgs.Exec()
		},
	}

	return cmd
}

// parseSSHFlags separates sogark-specific flags from ssh passthrough args.
// Returns: user, keyFormat, forceLogin, dryRun, host, sshExtraArgs, err.
// The first non-flag, non-sogark argument is treated as the host.
func parseSSHFlags(args []string) (user, keyFormat string, forceLogin, dryRun bool, host string, sshArgs []string, err error) {
	keyFormat = "openssh"
	i := 0
	hostFound := false
	for i < len(args) {
		a := args[i]
		switch {
		case a == "--verbose":
			os.Setenv("SOGARK_DEBUG", "1")
		case !hostFound && a == "--config":
			i++
			if i >= len(args) {
				err = fmt.Errorf(msg.FlagRequiresValue, a)
				return
			}
			config.FileOverride = args[i]
		case !hostFound && strings.HasPrefix(a, "--config="):
			config.FileOverride = strings.TrimPrefix(a, "--config=")
		case !hostFound && a == "--dry-run":
			dryRun = true
		case !hostFound && a == "--force-login":
			forceLogin = true
		case !hostFound && (a == "-u" || a == "--user"):
			i++
			if i >= len(args) {
				err = fmt.Errorf(msg.FlagRequiresValue, a)
				return
			}
			user = args[i]
		case !hostFound && strings.HasPrefix(a, "--user="):
			user = strings.TrimPrefix(a, "--user=")
		case !hostFound && a == "--key-format":
			i++
			if i >= len(args) {
				err = fmt.Errorf(msg.FlagRequiresValue, a)
				return
			}
			keyFormat = args[i]
		case !hostFound && strings.HasPrefix(a, "--key-format="):
			keyFormat = strings.TrimPrefix(a, "--key-format=")
		case a == "-h" || a == "--help":
			err = fmt.Errorf("help")
			return
		case !hostFound && a == "--":
			// skip separator, next non-flag is host
		case !hostFound && !strings.HasPrefix(a, "-"):
			host = a
			hostFound = true
		default:
			sshArgs = append(sshArgs, a)
		}
		i++
	}
	return
}

// doLogin performs Identity QR/push, the selected PVWA logon and key download.
func doLogin(cfg *config.Config) error {
	return doLoginWithFormats(cfg, cfg.KeyFormats)
}

func doLoginWithFormats(cfg *config.Config, formats []string) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	normalizedFormats, err := config.NormalizeKeyFormats(formats)
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
	cancel()

	fmt.Println(msg.DownloadingKeys)
	raw, err := client.FetchSSHKeys(signalCtx, normalizedFormats)
	if err != nil {
		return err
	}

	parsed, err := keys.Parse(raw)
	if err != nil {
		return err
	}
	if err := parsed.RequireFormats(normalizedFormats); err != nil {
		return err
	}

	keyDir, err := cfg.ResolveKeyDir()
	if err != nil {
		return err
	}

	results, err := keys.Save(parsed, keyDir, cfg.SSHKeyName, normalizedFormats)
	if err != nil {
		return err
	}

	if os.Getenv("SOGARK_DEBUG") != "" {
		for _, r := range results {
			info, err := os.Stat(r.Path)
			if err != nil {
				return fmt.Errorf("inspect saved key: %w", err)
			}
			fmt.Fprintf(os.Stderr, "[DEBUG] Saved %s (%d bytes, format=%s)\n", r.Path, info.Size(), r.Format)
		}
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
}

func formatDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}
