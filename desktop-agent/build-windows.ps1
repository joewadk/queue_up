$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $scriptDir

$iconPath = Join-Path $scriptDir "internal\appicon\queue_up.ico"
$sysoPath = Join-Path $scriptDir "cmd\queue-up-agent\queue_up.syso"
$binPath = Join-Path $scriptDir "queue-up-agent.exe"

if (-not (Test-Path $iconPath)) {
    throw "App icon not found: $iconPath"
}

if (Get-Command rsrc -ErrorAction SilentlyContinue) {
    & rsrc -ico $iconPath -o $sysoPath
    Write-Host "Embedded app icon resource generated at $sysoPath"
} else {
    Write-Host "rsrc not found; building without embedded exe icon. Install with: go install github.com/akavel/rsrc@latest"
}

go build -o $binPath .\cmd\queue-up-agent
Write-Host "Built $binPath"
