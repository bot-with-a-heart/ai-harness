# Configuration Guide

AI Harness reads configuration from `~/.ai-harness/config.toml` by default.

Use `--config <path>` on commands when testing an alternate config.

## Initialize

```powershell
ai-harness config init
ai-harness config show
ai-harness config doctor
```

## Default Shape

```toml
default_mode = "auto"

[providers.lmstudio.desktop]
type = "openai-compatible"
base_url = "http://127.0.0.1:1234/v1"
api_key = "lm-studio"
models = [
  "qwen2.5-coder-14b",
  "deepseek-coder",
]

[providers.codex.default]
type = "codex-cli"
profile = "default"

[routing]
local_first = true
escalate_on_failure = true

[memory.obsidian]
enabled = false
folder = "AI Harness"
export_history_limit = 20

[security]
enabled = true
required = false
key_provider = "os-keychain"
encrypt_history = true
encrypt_memory = true
encrypt_logs = true
retain_full_repo_context = false
recovery_exported = false
```

## LM Studio

Start the LM Studio local server and confirm the base URL in config.

```powershell
ai-harness models list
ai-harness ask-local "Say hello"
```

The model catalog is manually refreshed:

```powershell
ai-harness models catalog update
ai-harness models catalog show
```

Normal commands read the cached catalog rather than refreshing automatically.

## Codex

AI Harness shells out to the installed Codex CLI.

```powershell
ai-harness ask-codex "Review this repository"
```

Use `--profile`, `--sandbox`, and `--cd` on Codex-backed commands when needed.

## History

History is stored next to the active config:

```text
<config-dir>/history/
```

For the default config this is:

```text
~/.ai-harness/history/
```

After `security init`, history records are written as encrypted `.json.enc` files. Existing plaintext history can be migrated in place:

```powershell
ai-harness security init --provider passphrase --passphrase <passphrase> --required
ai-harness security migrate --passphrase <passphrase>
ai-harness security verify --passphrase <passphrase>
```

For the passphrase provider, commands can read `AI_HARNESS_PASSPHRASE` when `--passphrase` is omitted. The OS keychain provider stores the data key in the operating system credential store and does not write key material to config.

## Security

Security policy is stored in config, but encryption keys are not. The initial encrypted store is history. The memory and log encryption flags reserve policy for future canonical memory/log stores.

Useful commands:

```powershell
ai-harness security status
ai-harness security status --json
ai-harness security export-recovery --passphrase <passphrase> --output recovery.json
ai-harness security rotate-key --passphrase <old-passphrase> --new-passphrase <new-passphrase>
ai-harness security rotate-key --recovery-file recovery.json --new-passphrase <new-passphrase>
```

`retain_full_repo_context = false` keeps repository context summary-only by default. Any future full snapshot retention must be explicit and encrypted.

## Optional Obsidian Memory

Obsidian export is disabled by default and has no runtime dependency on Obsidian.

Configure a vault explicitly:

```powershell
ai-harness memory obsidian init --vault <vault-path>
```

Preview exports before writing:

```powershell
ai-harness memory obsidian export --dry-run
```

By default, the export writes managed notes under `AI Harness/`, exports model catalog notes when available, and excludes history. Use `--include-history` to add recent task outcome summaries. Task text remains redacted unless `--include-task-text` is passed.

## Logging

Global flags:

```powershell
ai-harness --log-level debug --log-json version
```

Logs are emitted to stderr. Command output remains on stdout.
