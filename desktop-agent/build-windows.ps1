$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $scriptDir

$iconPath = Join-Path $scriptDir "internal\appicon\queue_up.ico"
$sysoPath = Join-Path $scriptDir "cmd\queue-up-agent\queue_up.syso"
$binPath = Join-Path $scriptDir "queue-up-agent.exe"

if (-not (Test-Path $iconPath)) {
    throw "App icon not found: $iconPath"
}

${versionInfoDir} = Join-Path $scriptDir "cmd\queue-up-agent"
$versionInfoJson = Join-Path $versionInfoDir "versioninfo.json"
$versionInfoSyso = Join-Path $versionInfoDir "queue-up-agent.syso"
$legacyIconSyso = Join-Path $versionInfoDir "queue_up.syso"
$legacyRsrcSyso = Join-Path $versionInfoDir "rsrc_windows.syso"
$iconResourceEmbedded = $false

foreach ($legacy in @($legacyIconSyso, $legacyRsrcSyso, $versionInfoSyso)) {
    if (Test-Path $legacy) {
        Remove-Item -Force $legacy
    }
}

if (Test-Path $versionInfoJson) {
    if (Get-Command goversioninfo -ErrorAction SilentlyContinue) {
        Push-Location $versionInfoDir
        try {
            & goversioninfo -64 -icon $iconPath -o "queue-up-agent.syso"
        } finally {
            Pop-Location
        }

        Write-Host "Embedded version info resource generated at $versionInfoSyso"
        $iconResourceEmbedded = $true
    } else {
        Write-Host "goversioninfo not found; install with: go install github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest"
    }
}

if (-not $iconResourceEmbedded) {
    if (Get-Command rsrc -ErrorAction SilentlyContinue) {
        & rsrc -arch amd64 -ico $iconPath -o $sysoPath
        Write-Host "Embedded app icon resource generated at $sysoPath"
    } else {
        Write-Host "rsrc not found; building without embedded exe icon. Install with: go install github.com/akavel/rsrc@latest"
    }
}

$env:GOARCH = "amd64"
go build -o $binPath .\cmd\queue-up-agent
Write-Host "Built $binPath"
