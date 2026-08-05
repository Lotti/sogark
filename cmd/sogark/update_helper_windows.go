//go:build windows

package main

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/sys/windows"
)

func newUpdateHelperCmd() *cobra.Command {
	var (
		source  string
		target  string
		waitPID int
	)

	cmd := &cobra.Command{
		Use:    "__complete-update",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if source == "" || target == "" || waitPID <= 0 {
				return fmt.Errorf("missing update helper arguments")
			}

			if err := waitForProcessExit(waitPID, 60*time.Second); err != nil {
				return err
			}

			if err := replaceFileWithRetry(source, target, 60*time.Second); err != nil {
				return err
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&source, "source", "", "downloaded replacement executable")
	cmd.Flags().StringVar(&target, "target", "", "path of the running executable to replace")
	cmd.Flags().IntVar(&waitPID, "wait-pid", 0, "process ID to wait for before replacing")

	return cmd
}

func waitForProcessExit(pid int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := os.FindProcess(pid); err != nil {
			return nil
		}
		if !processExists(pid) {
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for process %d to exit", pid)
}

func processExists(pid int) bool {
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(handle)

	status, err := windows.WaitForSingleObject(handle, 0)
	if err != nil {
		return true
	}

	return status == uint32(windows.WAIT_TIMEOUT)
}
