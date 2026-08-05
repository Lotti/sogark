//go:build !windows

package main

import "github.com/spf13/cobra"

func newUpdateHelperCmd() *cobra.Command {
	return nil
}
