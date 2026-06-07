# Phase 10 Production Readiness

Phase 10 prepares AI Harness for internal sharing.

## Packaging

Build cross-platform binaries:

```powershell
.\scripts\build.ps1 -Version dev
```

Or on Linux/macOS:

```sh
VERSION=dev ./scripts/build.sh
```

Default targets:

```text
windows/amd64
linux/amd64
darwin/amd64
darwin/arm64
```

## Installers

Windows:

```powershell
.\install.ps1 -Build
.\install.ps1 -Build -AddToPath
.\install.ps1 -DryRun
```

Linux/macOS:

```sh
./install.sh --build
./install.sh --dry-run
```

Installers copy a user-local binary. PATH changes are opt-in on Windows and advisory on POSIX shells.

## Documentation

Production onboarding docs:

```text
docs/quick-start.md
docs/configuration.md
docs/troubleshooting.md
docs/architecture.md
```

## Logging

Global logging flags:

```powershell
ai-harness --log-level debug --log-json version
```

Supported levels:

```text
debug
info
warn
error
disabled
```

Logs are written to stderr so stdout remains compatible with wrappers, editors, and plugin integrations.

## Manual Testing

Verify:

```powershell
go test ./...
.\scripts\build.ps1 -Version smoke
.\install.ps1 -DryRun
.\ai-harness.exe --log-level debug --log-json version
.\ai-harness.exe history list
```

Expected:

```text
tests pass
cross-platform binaries build into dist/
installers explain what they will do in dry-run mode
logging emits structured records to stderr when enabled
normal command output remains on stdout
```

Completed before Phase 11.
