//go:build windows

package auth

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	msg "github.com/sogei/cyberark-cli/internal/messages"
)

// findPowerShell locates powershell.exe in PATH.
// Windows 10/11 always ships PowerShell 5.1 in PATH, so a simple LookPath is sufficient.
func findPowerShell() (string, error) {
	if p, err := exec.LookPath("powershell.exe"); err == nil {
		return p, nil
	}
	return "", fmt.Errorf(msg.AuthPSNotFound)
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

	fmt.Println(msg.AuthWindowOpening)
	fmt.Println(msg.AuthCompleteInWindow)

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
				return "", fmt.Errorf(msg.AuthSAMLFailed, stderr)
			}
		}
		return "", fmt.Errorf(msg.AuthSAMLFailedW, err)
	}

	samlResponse := strings.TrimSpace(string(output))
	if samlResponse == "" {
		return "", fmt.Errorf(msg.AuthSAMLEmpty)
	}

	fmt.Println(msg.AuthComplete)
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
    Write-Error "SAMLResponse not captured: login not completed?"
    exit 1
}
`
}
