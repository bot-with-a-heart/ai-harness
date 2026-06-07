param(
    [string]$InstallDir = "",
    [string]$BinaryPath = "",
    [switch]$Build,
    [switch]$AddToPath,
    [switch]$DryRun
)

$ErrorActionPreference = "Stop"

$repoRoot = Resolve-Path $PSScriptRoot
$go = if ($env:GO) { $env:GO } else { "go" }
if ([string]::IsNullOrWhiteSpace($InstallDir)) {
    $InstallDir = Join-Path $env:LOCALAPPDATA "ai-harness\bin"
}
if ([string]::IsNullOrWhiteSpace($BinaryPath)) {
    $BinaryPath = Join-Path $repoRoot "ai-harness.exe"
}

if ($Build) {
    $buildCmd = "$go build -trimpath -o `"$BinaryPath`" ./cmd/ai-harness"
    Write-Host "Build: $buildCmd"
    if (-not $DryRun) {
        Push-Location $repoRoot
        try {
            & $go build -trimpath -o $BinaryPath ./cmd/ai-harness
        }
        finally {
            Pop-Location
        }
    }
}

if (-not $DryRun -and -not (Test-Path $BinaryPath)) {
    throw "Binary not found at $BinaryPath. Run scripts\build.ps1 or pass -Build."
}

$targetPath = Join-Path $InstallDir "ai-harness.exe"
Write-Host "Install directory: $InstallDir"
Write-Host "Binary source: $BinaryPath"
Write-Host "Binary target: $targetPath"

if (-not $DryRun) {
    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
    Copy-Item -LiteralPath $BinaryPath -Destination $targetPath -Force
}

if ($AddToPath) {
    $currentPath = [Environment]::GetEnvironmentVariable("Path", "User")
    $entries = @()
    if ($currentPath) {
        $entries = $currentPath -split ";"
    }
    $alreadyPresent = $entries | Where-Object { $_ -ieq $InstallDir }
    if (-not $alreadyPresent) {
        Write-Host "Add to user PATH: $InstallDir"
        if (-not $DryRun) {
            $newPath = if ($currentPath) { "$currentPath;$InstallDir" } else { $InstallDir }
            [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
        }
    }
}
else {
    Write-Host "PATH was not modified. Re-run with -AddToPath to add $InstallDir to the user PATH."
}

if ($DryRun) {
    Write-Host "Dry run complete. No files or environment variables were changed."
}
else {
    Write-Host "Installed ai-harness to $targetPath"
}
