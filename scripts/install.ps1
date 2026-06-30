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

function Get-ExpectedChecksum {
    param(
        [Parameter(Mandatory = $true)][string]$ChecksumsPath,
        [Parameter(Mandatory = $true)][string]$FileName
    )

    foreach ($line in Get-Content -LiteralPath $ChecksumsPath) {
        if ([string]::IsNullOrWhiteSpace($line)) {
            continue
        }

        $parts = $line -split '\s+'
        if ($parts.Length -lt 2) {
            continue
        }

        $candidate = $parts[-1].TrimStart('*')
        if ($candidate -eq $FileName) {
            return $parts[0].ToLowerInvariant()
        }
    }

    throw "Checksum non trovata per $FileName"
}

function Test-FileChecksum {
    param(
        [Parameter(Mandatory = $true)][string]$Path,
        [Parameter(Mandatory = $true)][string]$Expected
    )

    $actual = (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($actual -ne $Expected.ToLowerInvariant()) {
        throw "Checksum non valida. Atteso: $Expected - Trovato: $actual"
    }
}

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
    $checksumsUrl = "$baseUrl/$Version/checksums.txt"
    Write-Host "[*] Download: $downloadUrl"
    Write-Host ""

    # Create install directory
    if (-not (Test-Path $InstallDir)) {
        New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    }

    # Download
    $destPath = Join-Path $InstallDir "sogark.exe"
    $tmpPath = Join-Path $InstallDir "sogark.exe.tmp"
    $checksumsPath = Join-Path $InstallDir "checksums.txt.tmp"

    try {
        Invoke-WebRequest -Uri $downloadUrl -OutFile $tmpPath -UseBasicParsing
        Invoke-WebRequest -Uri $checksumsUrl -OutFile $checksumsPath -UseBasicParsing
        $expectedChecksum = Get-ExpectedChecksum -ChecksumsPath $checksumsPath -FileName $binaryName
        Write-Host "[*] Verifica checksum..." -ForegroundColor Cyan
        Test-FileChecksum -Path $tmpPath -Expected $expectedChecksum
        Unblock-File -LiteralPath $tmpPath -ErrorAction SilentlyContinue
    } catch {
        Write-Host "[!] Errore download: $_" -ForegroundColor Red
        Remove-Item -Path $tmpPath, $checksumsPath -ErrorAction SilentlyContinue
        exit 1
    }
    Remove-Item -Path $checksumsPath -ErrorAction SilentlyContinue

    # Replace existing binary
    if (Test-Path $destPath) {
        Remove-Item -Path $destPath -Force
    }
    Move-Item -Path $tmpPath -Destination $destPath -Force
    Unblock-File -LiteralPath $destPath -ErrorAction SilentlyContinue

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
