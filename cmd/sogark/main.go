package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var version = "dev"

func main() {
	rootCmd := &cobra.Command{
		Use:     "sogark",
		Short:   "CyberArk PSMP CLI — autenticazione SAML/MFA e gestione sessioni SSH",
		Version: version,
	}

	rootCmd.AddCommand(
		newConnectCmd(),
		newLoginCmd(),
		newKeysCmd(),
		newConfigCmd(),
		newHostsCmd(),
		newMultiCmd(),
		newExecCmd(),
	)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
