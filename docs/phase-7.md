# Phase 7 Safe Edit Workflow

Phase 7 adds a controlled code modification workflow to `run`.

## Command

```powershell
.\ai-harness.exe run --edit "Add unit tests"
```

Useful options:

```powershell
.\ai-harness.exe run --edit --heuristic "Add unit tests"
.\ai-harness.exe run --edit --cd <repo-path> "Add unit tests"
.\ai-harness.exe run --edit --test-command "go test ./..." "Add unit tests"
.\ai-harness.exe run --edit --test-command "npm test" "Add component tests"
```

## Workflow

```text
Classify task
Choose provider
Collect repository context
Ask provider for unified diff only
Show generated diff
Require explicit user approval
Apply with git apply --check, then git apply
Run tests
```

## Safety Rules

The command never auto-applies a generated patch.

Approval requires typing:

```text
yes
```

Any other input leaves the workspace unchanged.

Generated responses must contain a valid-looking unified diff. Markdown-only explanations, shell commands, or malformed patch output are rejected before approval.

## Test Command Detection

`--test-command auto` is the default.

Auto detection currently uses:

```text
go.mod -> go test ./...
package.json -> npm test
pyproject.toml -> pytest
```

Use `--test-command` to override detection or to use a portable toolchain path.

## Manual Testing

Verify:

```powershell
.\ai-harness.exe run --edit --help
.\ai-harness.exe run --edit --heuristic --test-command "echo ok" "Add unit tests"
```

Expected:

```text
Generated diff is displayed
Approval prompt is shown
Typing anything other than yes does not apply the patch
Typing yes runs git apply --check, applies the patch, and runs tests
Test failures are reported
```

Completed before Phase 8.
