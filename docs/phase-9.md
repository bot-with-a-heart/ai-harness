# Phase 9 History and Audit Trail

Phase 9 records task executions to a local JSON history store.

## Storage

History records are stored beside the active config file:

```text
<config-dir>/history/
```

For the default config this resolves to:

```text
~/.ai-harness/history/
```

Each execution is stored as one JSON file named by its history ID.

## Commands

```powershell
.\ai-harness.exe history list
.\ai-harness.exe history list --json
.\ai-harness.exe history show <id>
.\ai-harness.exe history show <id> --json
```

## Tracked Commands

```text
classify
ask-local
ask-codex
run
run --edit
run --local-first
```

History inspection commands are not themselves recorded.

## Record Contents

Records include the task, command, provider, model, classification, touched files, tests run, escalation status, completion status, and error text when a command fails.

## Manual Testing

Verify:

```powershell
.\ai-harness.exe classify --heuristic "Explain this repository"
.\ai-harness.exe history list
.\ai-harness.exe history show <id>
```

Expected:

```text
classification command creates a history record
history list shows the new record
history show returns the full record
JSON output is available for integrations
```

Completed before Phase 10.
