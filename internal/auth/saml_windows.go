//go:build windows

package auth

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// findPowerShell locates powershell.exe, checking PATH first then the standard
// Windows system directory. This ensures it works from cmd.exe, PowerShell,
// MinGW64/MSYS2, Git Bash, and any other shell environment.
func findPowerShell() (string, error) {
	// Try PATH first (works in most cases)
	if p, err := exec.LookPath("powershell.exe"); err == nil {
		return p, nil
	}

	// Fallback: standard Windows PowerShell location
	sysRoot := os.Getenv("SystemRoot")
	if sysRoot == "" {
		sysRoot = `C:\Windows`
	}
	fullPath := filepath.Join(sysRoot, "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	if _, err := os.Stat(fullPath); err == nil {
		return fullPath, nil
	}

	return "", fmt.Errorf("powershell.exe non trovato.\n" +
		"Windows PowerShell è necessario per l'autenticazione SAML.")
}

// SAMLResponse captures the SAML response token using a WinForms embedded WebBrowser.
// This uses the exact same approach as PS-SAML-Interactive.psm1: a .NET WebBrowser
// control that intercepts the Navigating event and extracts the SAMLResponse from the
// HTML before the IDP auto-submits the form. No external browser process is needed.
func SAMLResponse(ctx context.Context, idpURL string, timeoutMinutes int) (string, error) {
	psPath, err := findPowerShell()
	if err != nil {
		return "", err
	}

	fmt.Println("[*] Apertura finestra di login SAML/MFA...")
	fmt.Println("   Completa l'autenticazione nella finestra.")

	script := buildPSScript(idpURL)

	cmd := exec.CommandContext(ctx, psPath,
		"-NoProfile",
		"-ExecutionPolicy", "Bypass",
		"-Command", script,
	)

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr := strings.TrimSpace(string(exitErr.Stderr))
			if stderr != "" {
				return "", fmt.Errorf("autenticazione SAML fallita: %s", stderr)
			}
		}
		return "", fmt.Errorf("autenticazione SAML fallita: %w", err)
	}

	samlResponse := strings.TrimSpace(string(output))
	if samlResponse == "" {
		return "", fmt.Errorf("SAMLResponse vuota: login non completato o finestra chiusa")
	}

	fmt.Println("[+] Autenticazione completata")
	return samlResponse, nil
}

// buildPSScript generates an inline PowerShell script that replicates the
// PS-SAML-Interactive.psm1 WinForms WebBrowser approach.
func buildPSScript(idpURL string) string {
	// Escape single quotes in URL for PowerShell string
	escapedURL := strings.ReplaceAll(idpURL, "'", "''")

	return `
Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Web

$RegEx = '(?i)name="SAMLResponse"(?: type="hidden")? value=\"(.*?)\"(?:.*)?\/>'
$SAMLResponse = $null

$form = New-Object Windows.Forms.Form
$form.StartPosition = [System.Windows.Forms.FormStartPosition]::CenterScreen
$form.Width = 640
$form.Height = 700
$form.ShowIcon = $false
$form.TopMost = $true
$form.Text = "sogark - Login SAML/MFA"

$web = New-Object Windows.Forms.WebBrowser
$web.Size = $form.ClientSize
$web.Anchor = "Left,Top,Right,Bottom"
$web.ScriptErrorsSuppressed = $true

$form.Controls.Add($web)
$web.Navigate('` + escapedURL + `')

$web.add_Navigating({
    if ($web.DocumentText -match "SAMLResponse") {
        $_.Cancel = $true
        if ($web.DocumentText -match $RegEx) {
            $Script:SAMLResponse = $(($Matches[1] -replace '&#x2b;', '+') -replace '&#x3d;', '=')
            $form.Close()
        }
    }
})

[System.Windows.Forms.Application]::Run($form)

if ($Script:SAMLResponse) {
    Write-Output $Script:SAMLResponse
} else {
    Write-Error "SAMLResponse non catturata: login non completato?"
    exit 1
}
`
}
