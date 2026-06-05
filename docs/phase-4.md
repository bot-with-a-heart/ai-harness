# Phase 4 Task Classification Agent

Phase 4 adds task classification for routing decisions.

## Objective

Use a local LM Studio model to classify a software development task and recommend either:

```text
lmstudio
codex
```

## Command

```powershell
.\ai-harness.exe classify "Refactor my AWS CDK application"
```

Default output is JSON:

```json
{
  "complexity": "high",
  "risk": "medium",
  "needsRepoAccess": true,
  "needsEdits": true,
  "needsTests": true,
  "recommendedProvider": "codex",
  "reason": "Architecture-level refactor should use Codex."
}
```

Useful options:

```powershell
.\ai-harness.exe classify --model <model-id> "Add tests for this middleware"
.\ai-harness.exe classify --summary "Explain what this repository does"
.\ai-harness.exe classify --heuristic "Refactor my AWS CDK application"
.\ai-harness.exe classify --no-fallback "Review this repository"
```

`--heuristic` skips LM Studio and uses deterministic local rules. Normal classification calls LM Studio first.

If LM Studio returns invalid classification JSON, the command falls back to deterministic rules unless `--no-fallback` is set.

## Routing Logic

LM Studio is preferred for:

```text
explanations
documentation
code review
small functions
simple test generation
```

Codex is preferred for:

```text
multi-file changes
architecture work
security-sensitive changes
complex debugging
failed local attempts
```

## Manual Testing

Verify:

```powershell
.\ai-harness.exe classify "Explain what this repository does"
.\ai-harness.exe classify "Refactor my AWS CDK application"
.\ai-harness.exe classify --heuristic --summary "Fix authentication middleware"
```

Expected:

```text
Explanation tasks route to lmstudio
Architecture/security/debugging tasks route to codex
JSON output parses cleanly
Heuristic fallback is available
```

Completed before Phase 5.
