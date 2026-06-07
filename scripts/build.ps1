param(
    [string]$Version = "dev",
    [string]$OutputDir = "dist",
    [string[]]$Targets = @("windows/amd64", "linux/amd64", "darwin/amd64", "darwin/arm64"),
    [switch]$Clean
)

$ErrorActionPreference = "Stop"

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
$distRoot = Join-Path $repoRoot $OutputDir
$go = if ($env:GO) { $env:GO } else { "go" }

if ($Clean -and (Test-Path $distRoot)) {
    Remove-Item -LiteralPath $distRoot -Recurse -Force
}
New-Item -ItemType Directory -Force -Path $distRoot | Out-Null

$oldGOOS = $env:GOOS
$oldGOARCH = $env:GOARCH
$oldCGO = $env:CGO_ENABLED

try {
    foreach ($target in $Targets) {
        $parts = $target -split "/"
        if ($parts.Count -ne 2) {
            throw "Invalid target '$target'. Expected GOOS/GOARCH."
        }

        $goos = $parts[0]
        $goarch = $parts[1]
        $targetName = "ai-harness-$Version-$goos-$goarch"
        $targetDir = Join-Path $distRoot $targetName
        New-Item -ItemType Directory -Force -Path $targetDir | Out-Null

        $binary = if ($goos -eq "windows") { "ai-harness.exe" } else { "ai-harness" }
        $output = Join-Path $targetDir $binary

        $env:GOOS = $goos
        $env:GOARCH = $goarch
        $env:CGO_ENABLED = "0"

        & $go build -trimpath -ldflags "-s -w -X main.version=$Version" -o $output ./cmd/ai-harness
        if ($LASTEXITCODE -ne 0) {
            throw "go build failed for $target"
        }

        Write-Host "Built $output"
    }
}
finally {
    $env:GOOS = $oldGOOS
    $env:GOARCH = $oldGOARCH
    $env:CGO_ENABLED = $oldCGO
}
