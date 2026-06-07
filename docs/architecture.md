# Architecture Overview

AI Harness is organized around a small CLI core and internal packages with clear ownership boundaries.

## Command Layer

```text
cmd/ai-harness
internal/cli
```

The Cobra command tree parses user intent, configures logging, and calls internal packages. Command stdout is reserved for command results. Logs go to stderr.

## Configuration

```text
internal/config
```

Configuration is TOML-based and validated before use. The config package owns default paths, initialization, and doctor checks.

## Providers

```text
internal/providers
internal/providers/lmstudio
internal/providers/codex
```

Providers implement a shared interface for health checks, model listing, and prompt execution.

LM Studio uses an OpenAI-compatible HTTP API. Codex uses the installed Codex CLI through a subprocess.

## Classification And Routing

```text
internal/classification
internal/router
```

The classifier decides task complexity, risk, repository needs, edit needs, test needs, and recommended provider. The router turns that decision into a provider call.

## Repository Context

```text
internal/context
```

The context collector gathers repository metadata, language/framework signals, key files, git status, and optional git diff while excluding sensitive and generated paths.

## Safe Edit Workflow

```text
internal/patch
```

Patch workflows require unified diffs, explicit user approval, `git apply --check`, `git apply`, and test execution. Local-first escalation sends Codex the local failure and current diff.

## Model Catalog

```text
internal/catalog
```

The model catalog is refreshed manually from LM Studio and cached locally. Normal routing can read the cached model knowledge without scanning LM Studio every run.

## History

```text
internal/history
```

Task executions are serialized by the history package. Records capture task text, selected provider, model, classification, touched files, tests, escalation status, and command outcome.

## Security

```text
internal/security
internal/cli security
```

The security package owns encryption-at-rest primitives, key resolution, encrypted history storage, migration, verification, recovery export, and key rotation. Encrypted history records are stored as AES-256-GCM JSON envelopes with a random nonce per record and authenticated metadata. Plaintext history remains supported only for pre-migration or uninitialized configs.

Security status is available as structured JSON so VS Code, Codex, Claude Code, and similar integrations can detect locked, initialized, required, and recoverable states without scraping human text.

## Optional Obsidian Export

```text
internal/obsidian
internal/cli memory obsidian
```

Obsidian export is an optional Markdown projection of selected harness memory. It writes managed notes with frontmatter into an explicitly configured vault folder, redacts history task text by default, and refuses to overwrite user-edited notes unless forced.

Canonical history and catalog data remain in harness-owned local storage.

## Packaging

```text
scripts/build.ps1
scripts/build.sh
install.ps1
install.sh
```

Build scripts produce cross-platform binaries. Installers copy a binary into a user-local directory and avoid PATH changes unless requested.
