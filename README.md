# AI Harness

AI Harness is a local-first AI coding harness planned to route software development tasks between local models, Codex, and future providers.

This repository is currently stopped for review after Phase 13: encryption-at-rest implementation.

## Current Commands

```powershell
.\ai-harness.exe version
.\ai-harness.exe help
.\ai-harness.exe config init
.\ai-harness.exe config show
.\ai-harness.exe config doctor
.\ai-harness.exe context
.\ai-harness.exe classify "Refactor my AWS CDK application"
.\ai-harness.exe run "Add tests for the authentication middleware"
.\ai-harness.exe run --edit "Add unit tests"
.\ai-harness.exe run --local-first "Fix failing tests"
.\ai-harness.exe history list
.\ai-harness.exe history show <id>
.\ai-harness.exe security status
.\ai-harness.exe security init --provider passphrase --passphrase <passphrase> --required
.\ai-harness.exe security migrate --passphrase <passphrase>
.\ai-harness.exe security verify --passphrase <passphrase>
.\ai-harness.exe security export-recovery --passphrase <passphrase> --output recovery.json
.\ai-harness.exe security rotate-key --passphrase <old-passphrase> --new-passphrase <new-passphrase>
.\ai-harness.exe security rotate-key --recovery-file recovery.json --new-passphrase <new-passphrase>
.\ai-harness.exe memory obsidian status
.\ai-harness.exe memory obsidian init --vault <vault-path>
.\ai-harness.exe memory obsidian export --dry-run
.\ai-harness.exe models list
.\ai-harness.exe ask-local "Explain what this repository does"
.\ai-harness.exe ask-codex "Review this repository"
.\ai-harness.exe models catalog update
.\ai-harness.exe models catalog show
.\ai-harness.exe models catalog explain <model-id>
.\ai-harness.exe --log-level debug --log-json version
```

## Production Readiness

```powershell
.\scripts\build.ps1 -Version dev
.\install.ps1 -DryRun
```

## Documentation

Start with [docs/quick-start.md](docs/quick-start.md), then see [docs/configuration.md](docs/configuration.md), [docs/troubleshooting.md](docs/troubleshooting.md), and [docs/architecture.md](docs/architecture.md).

## Roadmap

Future phases are tracked in [docs/roadmap.md](docs/roadmap.md).
