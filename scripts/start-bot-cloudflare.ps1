$ErrorActionPreference = "Stop"

$RepoRoot = Split-Path -Parent $PSScriptRoot
$FrontendDir = Join-Path $RepoRoot "web"
$Domain = "https://bot.huyche.site"

function Test-PortListening {
    param([int]$Port)
    return [bool](Get-NetTCPConnection -State Listen -LocalPort $Port -ErrorAction SilentlyContinue)
}

function Resolve-YarnCmd {
    $cmd = Get-Command yarn.cmd -ErrorAction SilentlyContinue
    if ($cmd) {
        return $cmd.Source
    }

    $knownPaths = @(
        "C:\nvm4w\nodejs\yarn.CMD",
        (Join-Path $env:APPDATA "npm\yarn.cmd")
    )

    foreach ($path in $knownPaths) {
        if ($path -and (Test-Path $path)) {
            return $path
        }
    }

    throw "Yarn was not found. Install or enable Yarn, then run this launcher again."
}

function Start-HfxBackground {
    param(
        [string]$Title,
        [string]$FilePath,
        [string[]]$ArgumentList = @(),
        [string]$WorkingDirectory,
        [string]$StdoutPath,
        [string]$StderrPath
    )

    $startArgs = @{
        FilePath               = $FilePath
        WorkingDirectory       = $WorkingDirectory
        RedirectStandardOutput = $StdoutPath
        RedirectStandardError  = $StderrPath
        WindowStyle            = "Hidden"
        PassThru               = $true
    }

    if ($ArgumentList.Count -gt 0) {
        $startArgs.ArgumentList = $ArgumentList
    }

    $process = Start-Process @startArgs

    Write-Host "Started $Title (PID $($process.Id))."
}

$YarnCmd = Resolve-YarnCmd
$DataDir = Join-Path $RepoRoot "data"
if (-not (Test-Path $DataDir)) {
    New-Item -ItemType Directory -Path $DataDir | Out-Null
}

Write-Host "HFX Cloudflare Tunnel launcher" -ForegroundColor Cyan
Write-Host "Repo: $RepoRoot"
Write-Host "Domain: $Domain"
Write-Host ""

$cloudflaredService = Get-Service cloudflared -ErrorAction SilentlyContinue
if ($cloudflaredService) {
    Write-Host "cloudflared service: $($cloudflaredService.Status)"
    if ($cloudflaredService.Status -ne "Running") {
        Write-Host "Starting cloudflared service..."
        Start-Service cloudflared
    }
} else {
    Write-Host "cloudflared service was not found. Install it from Cloudflare first." -ForegroundColor Yellow
}

if (Test-PortListening 8080) {
    Write-Host "Port 8080 is already listening. Backend may already be running." -ForegroundColor Yellow
} else {
    $backendExe = Join-Path $RepoRoot "hfx-server.exe"
    Write-Host "Building backend binary..."
    Push-Location $RepoRoot
    try {
        go build -o $backendExe .
    } finally {
        Pop-Location
    }

    Start-HfxBackground `
        -Title "backend :8080" `
        -FilePath $backendExe `
        -WorkingDirectory $RepoRoot `
        -StdoutPath (Join-Path $DataDir "backend.out.log") `
        -StderrPath (Join-Path $DataDir "backend.err.log")
}

if (Test-PortListening 3000) {
    Write-Host "Port 3000 is already listening. Frontend may already be running." -ForegroundColor Yellow
} else {
    Start-HfxBackground `
        -Title "frontend :3000" `
        -FilePath $YarnCmd `
        -ArgumentList @("dev", "--host", "0.0.0.0") `
        -WorkingDirectory $FrontendDir `
        -StdoutPath (Join-Path $DataDir "frontend.out.log") `
        -StderrPath (Join-Path $DataDir "frontend.err.log")
}

Write-Host ""
Write-Host "When Cloudflare route is configured, open: $Domain" -ForegroundColor Green
Write-Host "Local frontend: http://127.0.0.1:3000" -ForegroundColor DarkGray
Write-Host "Logs: $DataDir" -ForegroundColor DarkGray
