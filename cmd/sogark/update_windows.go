//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	msg "github.com/Lotti/sogark/internal/messages"
)

func replaceCurrentBinary(execPath, tmpPath, _ string) (binaryReplaceResult, error) {
	psPath, err := exec.LookPath("powershell.exe")
	if err != nil {
		return binaryReplaceResult{}, fmt.Errorf(msg.UpdateErrReplace, err)
	}

	scriptPath := filepath.Join(os.TempDir(), fmt.Sprintf("sogark-update-%d.ps1", os.Getpid()))
	script := fmt.Sprintf(`$ErrorActionPreference = "Stop"
$Target = '%s'
$Source = '%s'
$PidToWait = %d

for ($i = 0; $i -lt 240; $i++) {
    if (-not (Get-Process -Id $PidToWait -ErrorAction SilentlyContinue)) {
        break
    }
    Start-Sleep -Milliseconds 250
}

if (Test-Path -LiteralPath $Target) {
    Remove-Item -LiteralPath $Target -Force
}
Move-Item -LiteralPath $Source -Destination $Target -Force
try {
    Unblock-File -LiteralPath $Target -ErrorAction Stop
} catch {
}
`, psSingleQuote(execPath), psSingleQuote(tmpPath), os.Getpid())

	if err := os.WriteFile(scriptPath, []byte(script), 0600); err != nil {
		return binaryReplaceResult{}, fmt.Errorf(msg.UpdateErrReplace, err)
	}

	cmd := exec.Command(psPath, "-NoProfile", "-ExecutionPolicy", "Bypass", "-WindowStyle", "Hidden", "-File", scriptPath)
	if err := cmd.Start(); err != nil {
		return binaryReplaceResult{}, fmt.Errorf(msg.UpdateErrReplace, err)
	}

	return binaryReplaceResult{Deferred: true}, nil
}

func psSingleQuote(value string) string {
	return strings.ReplaceAll(value, `'`, `''`)
}
