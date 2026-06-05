# Phase 3 Codex Provider

Phase 3 adds Codex CLI support through a subprocess provider.

## Design

Authentication is intentionally out of scope. The harness uses the installed `codex` executable and the user's existing Codex CLI configuration.

The provider executes:

```powershell
codex exec --cd <repo> --sandbox <mode> --ephemeral --color never --output-last-message <temp-file> "<prompt>"
```

The harness reads the final response from `--output-last-message` and falls back to stdout/stderr if that file is empty. If Codex writes a final message and then exits non-zero during cleanup/session recording, the harness still returns the captured final message.

When a named profile is configured, the harness also passes `--profile <profile>`. The value `default` is treated as the Codex CLI default and is not passed as a named profile.

## Command

```powershell
.\ai-harness.exe ask-codex "Review this repository"
```

Useful options:

```powershell
.\ai-harness.exe ask-codex --provider default --profile default "Review this repository"
.\ai-harness.exe ask-codex --model <model> "Review this repository"
.\ai-harness.exe ask-codex --sandbox read-only "Review this repository"
.\ai-harness.exe ask-codex --cd <repo-path> "Review this repository"
```

## Manual Testing

Verify:

```powershell
.\ai-harness.exe config doctor
.\ai-harness.exe ask-codex "Reply with exactly: codex-ok"
```

Expected:

```text
Codex is detected
codex exec runs successfully
Final output is captured and printed
Errors include useful subprocess output
```

Completed before Phase 4.
