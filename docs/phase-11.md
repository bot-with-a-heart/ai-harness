# Phase 11 Optional Obsidian Vault Memory Integration

Phase 11 adds optional Obsidian vault support as a human-readable memory layer.

## Objective

Allow AI Harness to export or mirror selected memory into an Obsidian vault without making Obsidian required for normal operation.

## Design Rule

Obsidian is optional.

The harness must continue to work when:

```text
Obsidian is not installed
No vault path is configured
The vault is unavailable
The user disables Obsidian integration
```

## Intended Role

Obsidian should be used for curated, inspectable knowledge such as:

```text
project summaries
model suitability notes
routing lessons learned
architecture notes
task outcome summaries
manual user annotations
cross-project memory
```

Obsidian should not be the only source of truth for:

```text
execution history
audit records
security-sensitive logs
machine-critical state
```

## Commands

```powershell
.\ai-harness.exe memory obsidian init
.\ai-harness.exe memory obsidian export
.\ai-harness.exe memory obsidian status
```

Useful options:

```powershell
.\ai-harness.exe memory obsidian init --vault <vault-path>
.\ai-harness.exe memory obsidian init --vault <vault-path> --folder "AI Harness"
.\ai-harness.exe memory obsidian init --vault <vault-path> --create-vault
.\ai-harness.exe memory obsidian export --dry-run
.\ai-harness.exe memory obsidian export --vault <vault-path> --dry-run
.\ai-harness.exe memory obsidian export --include-history
.\ai-harness.exe memory obsidian export --include-history --include-task-text
.\ai-harness.exe memory obsidian export --force
```

## Configuration

Obsidian integration is disabled by default:

```toml
[memory.obsidian]
enabled = false
folder = "AI Harness"
export_history_limit = 20
```

`memory obsidian init --vault <path>` enables the integration and records the vault path.

## Export Behavior

```text
write Markdown notes to a configured vault folder
use predictable folder and filename conventions
include frontmatter for machine-readable metadata
redact task text from history exports by default
avoid overwriting user-edited notes without detection
keep canonical harness records in the harness-owned store
```

Default note paths:

```text
AI Harness/Index.md
AI Harness/Models/Model Catalog.md
AI Harness/History/Recent Task Outcomes.md
```

`Recent Task Outcomes.md` is only exported when `--include-history` is used.

Existing files are overwritten only when they contain:

```yaml
ai_harness_managed: true
```

User-edited conflicts are reported and refused unless `--force` is provided.

## Safety Requirements

Verify:

```text
integration is disabled by default
vault path must be explicit
sensitive memory classes are not exported by default
exports can be previewed before writing
conflicts are detected and reported
core CLI does not require Obsidian libraries or processes
```

The integration writes Markdown files only. It does not require Obsidian to be installed or running.

## Manual Testing

Verify:

```powershell
.\ai-harness.exe memory obsidian status
.\ai-harness.exe memory obsidian export --dry-run
.\ai-harness.exe memory obsidian export
```

Expected:

```text
no configured vault is handled gracefully
Markdown files are created only when configured and approved
frontmatter parses cleanly
user-edited notes are not silently destroyed
core harness commands still work without Obsidian
```

STOP FOR REVIEW before Phase 12.
