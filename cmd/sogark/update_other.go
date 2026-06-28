//go:build !windows

package main

import (
	"fmt"
	"os"

	msg "github.com/Lotti/sogark/internal/messages"
)

func replaceCurrentBinary(execPath, tmpPath, _ string) (binaryReplaceResult, error) {
	if err := os.Chmod(tmpPath, 0755); err != nil {
		return binaryReplaceResult{}, fmt.Errorf(msg.UpdateErrChmod, err)
	}
	if err := os.Rename(tmpPath, execPath); err != nil {
		return binaryReplaceResult{}, fmt.Errorf(msg.UpdateErrReplace, err)
	}
	return binaryReplaceResult{}, nil
}
