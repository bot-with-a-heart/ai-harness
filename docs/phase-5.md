# Phase 5 Unified Routing Engine

Phase 5 adds one command that classifies a task, chooses a provider, executes it, and returns the result.

## Command

```powershell
.\ai-harness.exe run "Add tests for the authentication middleware"
```

## Workflow

```text
Classify Task
Choose Provider
Execute Selected Provider
Return Results
```

The command prints:

```text
Provider Selected
Reason
Fallback Provider
Complexity
Risk
Needs repo access
Needs edits
Needs tests
Model
Provider response
```

## Useful Options

```powershell
.\ai-harness.exe run --heuristic "Explain what this repository does"
.\ai-harness.exe run --classifier-model <model-id> "Refactor this package"
.\ai-harness.exe run --local-model <model-id> "Explain this repository"
.\ai-harness.exe run --codex-model <model-id> "Review this repository"
.\ai-harness.exe run --sandbox read-only "Review this repository"
```

`--heuristic` skips LM Studio classification and uses deterministic local rules.

Fallback provider display is informational in Phase 5. The command does not automatically execute the fallback provider when the selected provider fails. Automatic fallback/escalation belongs to Phase 8.

## Manual Testing

Verify:

```powershell
.\ai-harness.exe run --heuristic "Explain what this repository does"
.\ai-harness.exe run --heuristic "Refactor my AWS CDK application"
.\ai-harness.exe run "Explain what this repository does"
```

Expected:

```text
Provider selection is displayed
Reason is displayed
Fallback provider is displayed
Selected provider executes
Result is returned
```

Completed before Phase 6.
