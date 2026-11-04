package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/Lotti/sogark/internal/config"
	"github.com/Lotti/sogark/internal/hosts"
	"github.com/Lotti/sogark/internal/keys"
	sshpkg "github.com/Lotti/sogark/internal/ssh"
	"github.com/spf13/cobra"
)

func newExecCmd() *cobra.Command {
	var options sshpkg.ExecuteOptions
	cmd := &cobra.Command{
		Use:   "exec [user@]host -- 'command'",
		Short: "Run a single command inside an interactive PSMP shell",
		Long:  "Opens an interactive SSH shell, waits for its prompt, sends a single-line POSIX shell command, then prints its combined output. Does not use SSH Command sessions. MFA remains interactive when keys need renewal.",
		Args:  cobra.ExactArgs(2),
		Example: `  sogark exec root@app01 -- 'id -u; hostname -f'
  sogark exec app01 --timeout 2m -- 'systemctl --no-pager status example'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.ArgsLenAtDash() != 1 {
				return errors.New("separate the host and a single quoted command with --")
			}
			if err := sshpkg.ValidateExecution(args[1], options); err != nil {
				return err
			}
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			target, err := resolveExecTarget(cfg, args[0])
			if err != nil {
				return err
			}
			dir, err := cfg.ResolveKeyDir()
			if err != nil {
				return err
			}
			valid, _, err := keys.IsValid(dir, cfg.SSHKeyName, cfg.KeyTTLHours)
			if err != nil {
				return err
			}
			if !valid {
				fmt.Fprintln(cmd.ErrOrStderr(), "SSH keys need renewal; interactive MFA is required")
				if err := doLogin(cfg); err != nil {
					return err
				}
			}
			ctx := signalCtx
			if ctx == nil {
				ctx = context.Background()
			}
			connection := sshpkg.ConnectArgs{
				Username: cfg.Username, TargetUser: target.TargetUser, Host: target.Address,
				ProxyHost: cfg.ProxyHost, KeyPath: filepath.Join(dir, cfg.SSHKeyName),
			}
			return connection.Execute(ctx, args[1], options, cmd.OutOrStdout())
		},
	}
	cmd.Flags().StringVar(&options.Prompt, "prompt", sshpkg.DefaultPromptPattern, "remote shell prompt regular expression")
	cmd.Flags().DurationVar(&options.ConnectTimeout, "connect-timeout", 90*time.Second, "time to wait for the remote shell prompt")
	cmd.Flags().DurationVar(&options.CommandTimeout, "timeout", 30*time.Second, "time to wait for command completion")
	return cmd
}

func resolveExecTarget(cfg *config.Config, argument string) (sshpkg.HostTarget, error) {
	user, host := sshpkg.ParseTarget(argument, cfg.DefaultSSHUser)
	if host == "" || user == "" || strings.ContainsAny(host, " \t\r\n@") {
		return sshpkg.HostTarget{}, errors.New("exec requires a host and a target username")
	}
	dir, err := config.Dir()
	if err != nil {
		return sshpkg.HostTarget{}, err
	}
	registry, err := hosts.NewRegistry(dir)
	if err != nil {
		return sshpkg.HostTarget{}, err
	}
	if registered, ok := registry.Get(host); ok {
		host = registered.Address
		if registered.User != "" && !strings.Contains(argument, "@") {
			user = registered.User
		}
	}
	return sshpkg.HostTarget{Name: argument, Address: host, TargetUser: user}, nil
}
