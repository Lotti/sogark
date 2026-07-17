package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

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

	rootCmd := &cobra.Command{
		Use:           "sogark",
		Short:         msg.RootShort,
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if verbose {
				os.Setenv("SOGARK_DEBUG", "1")
			}
			// Skip version check noise for the update command itself.
			if cmd.Name() != "update" {
				notifyIfUpdateAvailable()
				runBackgroundVersionCheck()
			}
		},
	}

	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, msg.RootFlagVerbose)

	rootCmd.AddCommand(
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
	)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
