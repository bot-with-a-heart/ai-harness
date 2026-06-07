# Quick Start

AI Harness is a local-first CLI for routing coding tasks between LM Studio, Codex CLI, and future providers.

## Install From Source

Build the binary:

```powershell
go build -trimpath -o ai-harness.exe ./cmd/ai-harness
```

Install on Windows:

```powershell
.\install.ps1 -Build -AddToPath
```

Install on Linux or macOS:

```sh
./install.sh --build
```

## Initialize Config

```powershell
ai-harness config init
ai-harness config doctor
```

The default config is stored in:

```text
~/.ai-harness/config.toml
```

Initialize encrypted history when you are ready to retain task records:

```powershell
ai-harness security init --provider passphrase --passphrase <passphrase> --required
ai-harness security status
```

Use `--provider os-keychain` instead when you want the operating system credential store to hold the data key.

## First Commands

Use local-only classification when setting up:

```powershell
ai-harness classify --heuristic --summary "Explain this repository"
```

List LM Studio models after LM Studio is running:

```powershell
ai-harness models list
```

Ask local LM Studio:

```powershell
ai-harness ask-local "Explain what this repository does"
```

Run routed execution:

```powershell
ai-harness run "Add tests for the authentication middleware"
```

Use safe edits:

```powershell
ai-harness run --edit "Add unit tests"
ai-harness run --local-first "Fix failing tests"
```

Inspect audit history:

```powershell
ai-harness history list
ai-harness history show <id>
```

If you initialized security with the passphrase provider, either pass `--passphrase` to security commands or set `AI_HARNESS_PASSPHRASE` before using commands that read encrypted history.

Preview optional Obsidian memory export:

```powershell
ai-harness memory obsidian status
ai-harness memory obsidian export --vault <vault-path> --dry-run
```

## Logging

Logs go to stderr so stdout stays usable for humans and integrations.

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
