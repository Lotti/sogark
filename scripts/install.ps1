# sogark installer for Windows (PowerShell)
# Usage: irm https://github.com/Lotti/sogark/releases/download/<version>/install.ps1 | iex
# Specific version: $env:VERSION='v1.2.0'; irm ... | iex

param(
    [string]$Version = $env:VERSION
)

$ErrorActionPreference = "Stop"

$UpdateRepo = "__UPDATE_REPO__"
if (-not $Version) { $Version = "latest" }

$InstallDir = Join-Path $env:USERPROFILE ".sogark\bin"

function Main {
    Write-Host "[*] Installazione sogark..." -ForegroundColor Cyan
    Write-Host ""

    # Detect architecture for Windows
    $arch = if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64') { "arm64" } else { "amd64" }

    $binaryName = "sogark-windows-$arch.exe"
    $baseUrl = "https://github.com/$UpdateRepo/releases/download"

    # Determine version
    if ($Version -eq "latest") {
        try {
            $apiUrl = "https://api.github.com/repos/$UpdateRepo/releases/latest"
            $release = Invoke-RestMethod -Uri $apiUrl
            $Version = $release.tag_name
            Write-Host "[*] Versione: $Version (latest)"
        } catch {
            Write-Host "[!] Impossibile determinare l'ultima versione. Specificare `$env:VERSION." -ForegroundColor Red
            exit 1
        }
    } else {
        Write-Host "[*] Versione: $Version"
    }

    $downloadUrl = "$baseUrl/$Version/$binaryName"
    Write-Host "[*] Download: $downloadUrl"
    Write-Host ""

    # Create install directory
    if (-not (Test-Path $InstallDir)) {
        New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    }

    # Download
    $destPath = Join-Path $InstallDir "sogark.exe"
    $tmpPath = Join-Path $InstallDir "sogark.exe.tmp"

    try {
        Invoke-WebRequest -Uri $downloadUrl -OutFile $tmpPath -UseBasicParsing
    } catch {
        Write-Host "[!] Errore download: $_" -ForegroundColor Red
        Remove-Item -Path $tmpPath -ErrorAction SilentlyContinue
        exit 1
    }

    # Replace existing binary
    if (Test-Path $destPath) {
        Remove-Item -Path $destPath -Force
    }
    Move-Item -Path $tmpPath -Destination $destPath -Force

    Write-Host "[✓] sogark installato in $destPath" -ForegroundColor Green

    # Set update_repo config for 'sogark update'
    try {
        & $destPath config set update_repo $UpdateRepo 2>$null
    } catch {
        # Config might not exist yet, that's fine
    }

    # Add to user PATH
    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if ($userPath -notlike "*$InstallDir*") {
        Write-Host ""
        Write-Host "[*] Aggiunta $InstallDir al PATH utente..." -ForegroundColor Cyan
        $newPath = "$InstallDir;$userPath"
        [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
        $env:Path = "$InstallDir;$env:Path"
        Write-Host "[✓] PATH aggiornato. Potrebbe essere necessario riavviare il terminale." -ForegroundColor Green
    } else {
        Write-Host "[✓] $InstallDir è già nel PATH" -ForegroundColor Green
    }

    Write-Host ""
    Write-Host "Per iniziare: sogark config init" -ForegroundColor Yellow
}

Main