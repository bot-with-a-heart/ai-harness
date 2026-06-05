# Phase 8 Escalation Workflow

Phase 8 adds local-first patch execution with Codex escalation.

## Command

```powershell
.\ai-harness.exe run --local-first "Fix failing tests"
```

Useful options:

```powershell
.\ai-harness.exe run --local-first --heuristic "Fix failing tests"
.\ai-harness.exe run --local-first --cd <repo-path> "Fix failing tests"
.\ai-harness.exe run --local-first --test-command "go test ./..." "Fix failing tests"
```

## Workflow

```text
Classify task
Attempt LM Studio patch
Show local diff
Require approval
Apply local patch
Run tests
If local generation, apply, or tests fail, escalate to Codex
Show Codex diff
Require approval
Apply Codex patch
Run tests
```

## Safety Rules

`--local-first` is an edit workflow. It never auto-applies a local or Codex patch.

Each patch attempt requires typing:

```text
yes
```

Declining a patch stops the workflow. It does not automatically escalate, because declining is treated as a user decision rather than a model failure.

If a local patch is approved and tests fail, the local patch remains in the working tree. The Codex escalation prompt includes the test failure and current git diff so Codex can produce a follow-up patch.

## Escalation Triggers

Codex escalation runs when the local attempt has one of these failures:

```text
provider execution error
missing or malformed unified diff
git apply --check or git apply failure
test command failure
```

Skipped tests are not treated as a failure.

## Manual Testing

Verify:

```powershell
.\ai-harness.exe run --local-first --help
.\ai-harness.exe run --local-first --heuristic --test-command "echo ok" "Fix failing tests"
```

Expected:

```text
Local attempt is shown first
Local diff requires approval
Passing local tests complete without Codex
Local failure prints the reason and escalates to Codex
Codex diff requires approval
Passing Codex tests complete the escalation
```

STOP FOR REVIEW before Phase 9.
