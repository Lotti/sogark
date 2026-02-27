<#
   ========================================================================================================================
   Name         : SAML_MFACache.ps1
   Description  : Scarica le chiavi private PPK (PuTTY), PEM e OpenSSH da Cyberark (MFACache)
   Created Date : 15/12/2025
   Created By   : gcriaco
   Dependencies : 1) PS-SAML-Interactive.psm1

   Revision History
   Date       Release  Change By      Description
   15/12/2025 1.0      gcriaco        Initial Release
   ========================================================================================================================
#>
import-module -name '.\PS-SAML-Interactive.psm1'

# ========== Parametri ==========
# NON MODIFICARE #
$PVWABaseURL  = "https://cyberark.sogei.it/PasswordVault"
$IDPURL = "https://aag4837.my.idaptive.app/login?yfirtnecapplogin=true&appKey=0f8346cb-fc6f-4ed4-9ebc-e2fcf5ae90c8&customerId=AAG4837&stateId=hFdfLAHPLkyZj2ml2B5cjMBjVjnT6AZd42pjywyZBoU1&yfirtnecrun=true"
$PVWAURI = "/API/auth/SAML/Logon/"
$KeyFormats = '{"formats":["PPK", "PEM", "OpenSSH"]}'
$InputFile  = $env:Temp + "\TempKeys.txt"

# Parametri modificabili #
$OutputDir = 'C:\Temp\'
$OutKeyOpenSSH = "key.openssh"
$OutKeyPPK = "key.PPK"
$OutKeyPEM = "key.PEM"

#===================================================================================================================================================
# FUNZIONI
#===================================================================================================================================================
# --- Normalizza un blocco chiave ---
function Normalize-KeyBlock {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory)]
        [string]$Text,

        # Se true: rimuove tutti gli spazi (attenzione: può corrompere strutture)
        [switch]$RemoveAllSpaces
    )

    # Suddividi in righe, trim per linea, rimuovi righe vuote duplicate
    $lines = $Text -split "`r?`n"

    if ($RemoveAllSpaces) {
        # Aggressivo: elimina tutti gli spazi ' ' e tab
        $lines = $lines | ForEach-Object { ($_ -replace '[ \t]+','').Trim() }
    }
    else {
        # Safe: solo trim inizio/fine riga
        $lines = $lines | ForEach-Object { $_.Trim() }
    }

    # Rimuovi righe completamente vuote consecutive, ma preserva una eventuale riga vuota singola
    $normalized = New-Object System.Collections.Generic.List[string]
    $previousEmpty = $false
    foreach ($line in $lines) {
        $isEmpty = [string]::IsNullOrWhiteSpace($line)
        if ($isEmpty) {
            if (-not $previousEmpty) {
                $normalized.Add('')
                $previousEmpty = $true
            }
        } else {
            $normalized.Add($line)
            $previousEmpty = $false
        }
    }

    # Ricompone con CRLF
    return ($normalized -join "`r`n").Trim()
}

# --- Scrittura file preservando newline (UTF-8 senza BOM) ---
function Write-AsciiFile {
    param(
        [Parameter(Mandatory)]
        [string]$Path,
        [Parameter(Mandatory)]
        [string]$Content,
        # Propaga l'opzione di rimozione totale degli spazi se richiesta
        [switch]$RemoveAllSpaces
    )
    $dir = Split-Path -Parent $Path
    if (-not (Test-Path -LiteralPath $dir)) {
        New-Item -ItemType Directory -Path $dir -Force | Out-Null
    }

    # Applica normalizzazione
    $Content = Normalize-KeyBlock -Text $Content -RemoveAllSpaces:$RemoveAllSpaces

    # UTF-8 senza BOM
    $utf8NoBom = New-Object System.Text.UTF8Encoding($false)
    [System.IO.File]::WriteAllText($Path, $Content, $utf8NoBom)
    return $Path
}

# --- Scrittura file con le chiavi ---
function Write-KeyFile {
    param(
        [Parameter(Mandatory)]
        [string]$InputFile,
        [Parameter(Mandatory)]
        [string]$OutputDir,
        [Parameter(Mandatory)]
        $OutKeyOpenSSH,
        [Parameter(Mandatory)]
        [string]$OutKeyPPK,
        [Parameter(Mandatory)]
        [string]$OutKeyPEM
    )

# Fail-fast sugli errori
$ErrorActionPreference = 'Stop'    
    
# --- Carica testo grezzo ---
if (-not (Test-Path $InputFile)) {
    throw "File non trovato: $InputFile"
}
$text = Get-Content $InputFile -Raw

# --- Pattern (here-strings per robustezza) ---

# 1) PPK (PuTTY): dalla intestazione fino alla fine dell'hash di Private-MAC,
# fermandosi SUBITO (lookahead) se segue ';' o newline/fine-file.
$ppkPattern = @'
(?ms)PuTTY-User-Key-File-\d+:[^\r\n]*\r?\n.*?Private-MAC:\s*[0-9a-fA-F]+(?=\s*(?:;|[\r\n]|$))
'@

# 2) PEM (RSA): blocco tra BEGIN/END
$pemPattern = @'
(?ms)-----BEGIN RSA PRIVATE KEY-----\s*.*?\s*-----END RSA PRIVATE KEY-----
'@

# 3) OpenSSH: blocco tra BEGIN/END
$opensshPattern = @'
(?ms)-----BEGIN OPENSSH PRIVATE KEY-----\s*.*?\s*-----END OPENSSH PRIVATE KEY-----
'@

# --- Estrai blocchi chiave ---
$ppkMatch     = [regex]::Match($text, $ppkPattern)
$ppkBlock     = if ($ppkMatch.Success) { $ppkMatch.Value.Trim() } else { $null }

$pemMatch     = [regex]::Match($text, $pemPattern)
$pemBlock     = if ($pemMatch.Success) { $pemMatch.Value.Trim() } else { $null }

$opensshMatch = [regex]::Match($text, $opensshPattern)
$opensshBlock = if ($opensshMatch.Success) { $opensshMatch.Value.Trim() } else { $null }

# --- Scrivi i file ---
$results = @()

if ($ppkBlock) {
    $path = Join-Path $OutputDir $OutKeyPPK
    Write-AsciiFile -Path $path -Content $ppkBlock | Out-Null
    $results += [pscustomobject]@{ Format='PPK'; File=$path }
    Write-Host "Chiave privata PPK estratta e salvata in: $path"

} else {
    Write-Warning "Blocco PPK non trovato nel file."
}

if ($pemBlock) {
    $path = Join-Path $OutputDir $OutKeyPEM
    Write-AsciiFile -Path $path -Content $pemBlock | Out-Null
    $results += [pscustomobject]@{ Format='PEM'; File=$path }
    Write-Host "Chiave privata PEM estratta e salvata in: $path"
} else {
    Write-Warning "Blocco PEM (RSA) non trovato nel file."
}

if ($opensshBlock) {
    $path = Join-Path $OutputDir $OutKeyOpenSSH
    Write-AsciiFile -Path $path -Content $opensshBlock | Out-Null
    $results += [pscustomobject]@{ Format='OpenSSH'; File=$path }
    Write-Host "Chiave privata OpenSSH estratta e salvata in: $path"
} else {
    Write-Warning "Blocco OpenSSH non trovato nel file."
}
# Write-Host $results
}

#===================================================================================================================================================
# MAIN
#===================================================================================================================================================
$SamlResponse = New-SAMLInteractive -LoginIDP $IDPURL

# SAML Logon
$Body        = @{
  apiUse           = 'true'
  concurrentSession= 'true'
  SAMLResponse     = $SamlResponse
}

$PVWAURL = $PVWABaseURL + $PVWAURI
try {
    $LoginToken = Invoke-RestMethod -Uri $PVWAURL `
        -Method Post -Body $Body -ContentType 'application/x-www-form-urlencoded'
} catch {
    Write-Error "Logon fallito: $($_.Exception.Message)"
    return
}

if ([string]::IsNullOrWhiteSpace($LoginToken)) {
    Write-Error "Token di sessione non ricevuto"
    return
}

# Get MFA cache
$Headers = @{ Authorization = $Logintoken }
$PVWAURI = "/API/Users/Secret/SSHKeys/Cache"
$PVWAURL = $PVWABaseURL + $PVWAURI
try {
    Invoke-RestMethod -Method Post -Uri $PVWAURL `
                  -Headers $Headers `
                  -ContentType "application/json" `
                  -Body $KeyFormats > $InputFile 
} catch {
    Write-Error "Generazione chiave MFA caching fallita: $($_.Exception.Message)"
    return
}

Write-KeyFile -InputFile "$InputFile" -OutputDir "$OutputDir" -OutKeyOpenSSH "$OutKeyOpenSSH" -OutKeyPPK "$OutKeyPPK" -OutKeyPEM "$OutKeyPEM"

Remove-Item $InputFile

Add-Type -AssemblyName System.Windows.Forms
$message = "$OutKeyOpenSSH`n$OutKeyPPK`n$OutKeyPEM`nATTENZIONE - le chiavi sono valide per quattro ore"

[System.Windows.Forms.MessageBox]::Show(
    $message,                                           # Text
    "Scarico Chiavi private in $OutputDir",             # Title
    [System.Windows.Forms.MessageBoxButtons]::OK,       # Buttons
    [System.Windows.Forms.MessageBoxIcon]::Information  # Icon
)