package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Lotti/sogark/internal/config"
	msg "github.com/Lotti/sogark/internal/messages"
	"github.com/spf13/cobra"
)

var (
	version = "dev"
	verbose bool
)

// signalCtx is a context cancelled on SIGINT/SIGTERM.
var signalCtx context.Context

func main() {
	var cancel context.CancelFunc
	signalCtx, cancel = signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Separate channel for the interrupt message — this way the goroutine
	// only prints on a real OS signal, not on the deferred cancel() at exit.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigCh
		fmt.Println(msg.RootInterrupted)
	}()

	rootCmd := newRootCmd()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:           "sogark",
		Short:         msg.RootShort,
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Name() == "__complete-update" {
				return nil
			}
			if cmd.Name() == "ssh" {
				if _, _, _, _, _, _, err := parseSSHFlags(args); err != nil {
					if err.Error() == "help" {
						return nil
					}
					return err
				}
			} else if cmd.Name() == "scp" {
				if _, err := parseScpFlags(args); err != nil {
					if err.Error() == "help" {
						return nil
					}
					return err
				}
			}
			wizard := cmd.Name() == "init" && cmd.Parent() != nil && cmd.Parent().Name() == "config"
			if !wizard {
				cfg, err := config.Load()
				if err != nil && !errors.Is(err, config.ErrConfigNotFound) {
					return err
				}
				configCommand := cmd.Parent() != nil && cmd.Parent().Name() == "config"
				if err == nil && !configCommand && cmd.Name() != "doctor" && cmd.Name() != "completion" {
					if _, err := cfg.SelectedAuthProfile(); err != nil {
						return fmt.Errorf("invalid active authentication profile; run 'sogark config init' (or 'sogark --config <file> config init' for a custom file): %w", err)
					}
				}
			}
			if verbose {
				os.Setenv("SOGARK_DEBUG", "1")
			}
			// Skip version check noise for the update command itself.
			if cmd.Name() != "update" {
				notifyIfUpdateAvailable()
				runBackgroundVersionCheck()
			}
			return nil
		},
	}

	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, msg.RootFlagVerbose)
	rootCmd.PersistentFlags().StringVar(&config.FileOverride, "config", "", "Configuration file (default: ~/.sogark/config.yaml)")

	commands := []*cobra.Command{
		newSSHCmd(),
		newScpCmd(),
		newLoginCmd(),
		newKeysCmd(),
		newConfigCmd(),
		newDoctorCmd(),
		newHostsCmd(),
		newMultiCmd(),
		newMobaCmd(),
		newWinSCPCmd(),
		newFileZillaCmd(),
		newUpdateCmd(),
		newCompletionCmd(),
	}
	if helperCmd := newUpdateHelperCmd(); helperCmd != nil {
		commands = append(commands, helperCmd)
	}
	rootCmd.AddCommand(commands...)

	return rootCmd
}
