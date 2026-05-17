#Requires -Version 5.1
<#
.SYNOPSIS
    Argus XDR Windows Installer

.DESCRIPTION
    Installs Argus XDR on Windows. Downloads the latest binary from GitHub,
    installs it to %LOCALAPPDATA%\Programs\argus, and adds it to the user PATH.

.EXAMPLE
    # Run from an elevated PowerShell prompt:
    irm https://raw.githubusercontent.com/kairos-dev-kairos-ecl/Argus/main/install.ps1 | iex

.PARAMETER Version
    Specific version to install (e.g. "v1.2.0"). Defaults to latest release.

.PARAMETER InstallDir
    Directory to install the argus binary. Defaults to $env:LOCALAPPDATA\Programs\argus

.PARAMETER NoPath
    Skip adding the install directory to the user PATH.
#>
param(
    [string]$Version   = "latest",
    [string]$InstallDir = "$env:LOCALAPPDATA\Programs\argus",
    [switch]$NoPath
)

$ErrorActionPreference = "Stop"

$GITHUB_REPO = "kairos-dev-kairos-ecl/Argus"
$BINARY_NAME = "argus-windows-amd64.exe"

# ─────────────────────────────────────────────────────────────────────────────
# Helpers
# ─────────────────────────────────────────────────────────────────────────────
function Write-Step  { param($msg) Write-Host "  → $msg" -ForegroundColor Cyan }
function Write-OK    { param($msg) Write-Host "  ✓ $msg" -ForegroundColor Green }
function Write-Warn  { param($msg) Write-Host "  ⚠ $msg" -ForegroundColor Yellow }
function Write-Fail  { param($msg) Write-Host "  ✗ $msg" -ForegroundColor Red; exit 1 }

# ─────────────────────────────────────────────────────────────────────────────
# 1. Architecture check
# ─────────────────────────────────────────────────────────────────────────────
Write-Host ""
Write-Host "  Argus XDR — Windows Installer" -ForegroundColor White
Write-Host "  ──────────────────────────────" -ForegroundColor DarkGray
Write-Host ""

$arch = (Get-WmiObject Win32_OperatingSystem).OSArchitecture
if ($arch -notmatch "64") {
    Write-Fail "Only 64-bit Windows is supported (detected: $arch)"
}
Write-OK "Architecture: x86_64"

# ─────────────────────────────────────────────────────────────────────────────
# 2. Resolve version
# ─────────────────────────────────────────────────────────────────────────────
Write-Step "Resolving version..."
if ($Version -eq "latest") {
    try {
        $release = Invoke-RestMethod "https://api.github.com/repos/$GITHUB_REPO/releases/latest" -ErrorAction Stop
        $Version = $release.tag_name
    } catch {
        Write-Fail "Failed to fetch latest release from GitHub: $_"
    }
}
Write-OK "Version: $Version"

# ─────────────────────────────────────────────────────────────────────────────
# 3. Download binary
# ─────────────────────────────────────────────────────────────────────────────
$binaryUrl   = "https://github.com/$GITHUB_REPO/releases/download/$Version/$BINARY_NAME"
$checksumUrl = "https://github.com/$GITHUB_REPO/releases/download/$Version/checksums.txt"
$tmpDir      = Join-Path $env:TEMP "argus-install-$([System.IO.Path]::GetRandomFileName())"
$tmpBinary   = Join-Path $tmpDir $BINARY_NAME

New-Item -ItemType Directory -Path $tmpDir -Force | Out-Null

Write-Step "Downloading $BINARY_NAME..."
try {
    Invoke-WebRequest -Uri $binaryUrl -OutFile $tmpBinary -UseBasicParsing -ErrorAction Stop
} catch {
    Write-Fail "Download failed: $_`n  URL: $binaryUrl"
}
Write-OK "Downloaded $BINARY_NAME"

# ─────────────────────────────────────────────────────────────────────────────
# 4. Verify checksum
# ─────────────────────────────────────────────────────────────────────────────
Write-Step "Verifying checksum..."
try {
    $checksumFile = Join-Path $tmpDir "checksums.txt"
    Invoke-WebRequest -Uri $checksumUrl -OutFile $checksumFile -UseBasicParsing -ErrorAction Stop

    $expected = (Get-Content $checksumFile | Where-Object { $_ -match $BINARY_NAME }) -split '\s+' | Select-Object -First 1
    $actual   = (Get-FileHash $tmpBinary -Algorithm SHA256).Hash.ToLower()

    if ($expected -and ($actual -ne $expected.ToLower())) {
        Write-Fail "Checksum mismatch!`n  Expected: $expected`n  Got:      $actual"
    }
    Write-OK "Checksum verified"
} catch {
    Write-Warn "Could not verify checksum (skipping): $_"
}

# ─────────────────────────────────────────────────────────────────────────────
# 5. Install binary
# ─────────────────────────────────────────────────────────────────────────────
Write-Step "Installing to $InstallDir..."
New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null

$dest = Join-Path $InstallDir "argus.exe"
Copy-Item $tmpBinary $dest -Force
Remove-Item $tmpDir -Recurse -Force -ErrorAction SilentlyContinue

Write-OK "Installed: $dest"

# ─────────────────────────────────────────────────────────────────────────────
# 6. Add to user PATH
# ─────────────────────────────────────────────────────────────────────────────
if (-not $NoPath) {
    $currentPath = [System.Environment]::GetEnvironmentVariable("PATH", "User")
    if ($currentPath -notlike "*$InstallDir*") {
        Write-Step "Adding $InstallDir to user PATH..."
        [System.Environment]::SetEnvironmentVariable("PATH", "$currentPath;$InstallDir", "User")
        # Also update current session
        $env:PATH = "$env:PATH;$InstallDir"
        Write-OK "PATH updated"
    } else {
        Write-OK "PATH already contains $InstallDir"
    }
}

# ─────────────────────────────────────────────────────────────────────────────
# 7. Verify installation
# ─────────────────────────────────────────────────────────────────────────────
Write-Step "Verifying installation..."
try {
    $ver = & $dest --version 2>&1 | Select-Object -First 1
    Write-OK "argus $ver"
} catch {
    Write-Warn "Could not run 'argus --version' — restart your terminal and try again"
}

# ─────────────────────────────────────────────────────────────────────────────
# 8. Next steps
# ─────────────────────────────────────────────────────────────────────────────
Write-Host ""
Write-Host "  ╔══════════════════════════════════════════════════╗" -ForegroundColor DarkGray
Write-Host "  ║  Argus XDR $Version installed successfully        " -ForegroundColor White
Write-Host "  ╚══════════════════════════════════════════════════╝" -ForegroundColor DarkGray
Write-Host ""
Write-Host "  Quick start:" -ForegroundColor White
Write-Host "    docker compose up -d        " -ForegroundColor Yellow -NoNewline
Write-Host "# start ClickHouse + PostgreSQL + Redis" -ForegroundColor DarkGray
Write-Host "    argus server start          " -ForegroundColor Yellow -NoNewline
Write-Host "# start Argus" -ForegroundColor DarkGray
Write-Host "    argus doctor                " -ForegroundColor Yellow -NoNewline
Write-Host "# verify everything is connected" -ForegroundColor DarkGray
Write-Host ""
Write-Host "  Dashboard:  " -ForegroundColor DarkGray -NoNewline
Write-Host "http://localhost:8080" -ForegroundColor Cyan
Write-Host "  Docs:       " -ForegroundColor DarkGray -NoNewline
Write-Host "https://github.com/kairos-dev-kairos-ecl/Argus/tree/main/docs" -ForegroundColor Cyan
Write-Host ""
Write-Host "  Restart your terminal for PATH changes to take effect." -ForegroundColor DarkGray
Write-Host ""
