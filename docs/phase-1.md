# Phase 1 Configuration System

Phase 1 adds the local configuration file and doctor checks.

## Default Paths

Windows:

```text
%USERPROFILE%\.ai-harness\config.toml
%USERPROFILE%\.ai-harness\history\
```

Linux/macOS:

```text
~/.ai-harness/config.toml
~/.ai-harness/history/
```

## Commands

```powershell
.\ai-harness.exe config init
.\ai-harness.exe config show
.\ai-harness.exe config doctor
```

`config init` creates a starter config with a local LM Studio endpoint at `http://127.0.0.1:1234/v1` and a default Codex CLI profile.

## Review Gate

Stop after build, command verification, and review before starting Phase 2.
