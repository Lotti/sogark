//go:build windows

package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	msg "github.com/Lotti/sogark/internal/messages"
)

func replaceCurrentBinary(execPath, tmpPath, _ string) (binaryReplaceResult, error) {
	helperPath := filepath.Join(os.TempDir(), fmt.Sprintf("sogark-update-helper-%d.exe", os.Getpid()))
	if err := copyFile(execPath, helperPath); err != nil {
		return binaryReplaceResult{}, fmt.Errorf(msg.UpdateErrReplace, err)
	}

	cmd := exec.Command(helperPath,
		"__complete-update",
		"--source", tmpPath,
		"--target", execPath,
		"--wait-pid", strconv.Itoa(os.Getpid()),
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err != nil {
		return binaryReplaceResult{}, fmt.Errorf(msg.UpdateErrReplace, err)
	}

	return binaryReplaceResult{Deferred: true}, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}

	return nil
}

func replaceFileWithRetry(source, target string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		_ = os.Remove(target)
		if err := os.Rename(source, target); err == nil {
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	return fmt.Errorf("timeout replacing %s", target)
}
