//go:build !windows

package main

import (
	"os/exec"
	"runtime"
)

func clearTrustMetadata(path string) error {
	if runtime.GOOS != "darwin" {
		return nil
	}

	cmd := exec.Command("xattr", "-d", "com.apple.quarantine", path)
	if err := cmd.Run(); err != nil {
		return nil
	}

	return nil
}
