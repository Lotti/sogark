package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

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
		fmt.Println("\n[!] Operazione interrotta")
	}()

	rootCmd := &cobra.Command{
		Use:     "sogark",
		Short:   "CyberArk PSMP CLI — autenticazione SAML/MFA e gestione sessioni SSH",
		Version: version,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if verbose {
				os.Setenv("SOGARK_DEBUG", "1")
			}
		},
	}

	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "output dettagliato per debug")

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
		newCompletionCmd(),
	)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
