# sogark installer for Windows (PowerShell)
# Usage: irm <nexus_url>/repository/<repo>/latest/install.ps1 | iex
# Specific version: $env:VERSION='v1.2.0'; irm ... | iex

param(
    [string]$Version = $env:VERSION,
    [string]$NexusUrl = $env:NEXUS_URL
)

$ErrorActionPreference = "Stop"

if (-not $NexusUrl) { $NexusUrl = "__NEXUS_URL__" }
$NexusRepo = "__NEXUS_REPO__"
if (-not $Version) { $Version = "latest" }

$InstallDir = Join-Path $env:USERPROFILE ".sogark\bin"

function Main {
    Write-Host "[*] Installazione sogark..." -ForegroundColor Cyan
    Write-Host ""

    $binaryName = "sogark-windows-amd64.exe"
    $downloadUrl = "$NexusUrl/repository/$NexusRepo/$Version/$binaryName"

    # Show version
    if ($Version -eq "latest") {
        try {
            $ver = (Invoke-WebRequest -Uri "$NexusUrl/repository/$NexusRepo/latest/version.txt" -UseBasicParsing).Content.Trim()
            Write-Host "[*] Versione: $ver (latest)"
        } catch {
            Write-Host "[*] Versione: latest"
        }
    } else {
        Write-Host "[*] Versione: $Version"
    }
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

    # Set nexus config for 'sogark update'
    try {
        & $destPath config set nexus_url $NexusUrl 2>$null
        & $destPath config set nexus_repo $NexusRepo 2>$null
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
