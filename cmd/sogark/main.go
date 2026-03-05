package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	msg "github.com/sogei/cyberark-cli/internal/messages"
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

	go func() {
		<-signalCtx.Done()
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
		},
	}

	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, msg.RootFlagVerbose)

	rootCmd.AddCommand(
		newSSHCmd(),
		newScpCmd(),
		newLoginCmd(),
		newKeysCmd(),
		newConfigCmd(),
		newHostsCmd(),
		newMultiCmd(),
		newMobaCmd(),
		newWinSCPCmd(),
		newUpdateCmd(),
		newCompletionCmd(),
	)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
