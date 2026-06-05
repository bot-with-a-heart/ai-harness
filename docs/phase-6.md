# Phase 6 Repository Context Engine

Phase 6 adds repository intelligence collection.

## Command

```powershell
.\ai-harness.exe context
```

Useful options:

```powershell
.\ai-harness.exe context --json
.\ai-harness.exe context --path <repo-path>
.\ai-harness.exe context --max-depth 4 --max-files 500
.\ai-harness.exe context --max-file-bytes 32768
.\ai-harness.exe context --max-diff-bytes 131072
.\ai-harness.exe context --no-diff
```

## Gathered Data

The context command gathers:

```text
README
package.json
go.mod
pyproject.toml
git status
git diff
directory structure
languages
frameworks and libraries
```

## Sensitive Exclusions

The collector skips common sensitive or noisy paths, including:

```text
.git
.env
.codex
.gotmp
.gocache
node_modules
vendor
dist
build
bin
*.exe artifacts
secrets
credentials
private keys
certificate/key files
```

Key file contents and git diff output are bounded by byte limits and marked when truncated.

## Manual Testing

Verify:

```powershell
.\ai-harness.exe context
.\ai-harness.exe context --json
.\ai-harness.exe context --max-depth 2 --max-files 100 --no-diff
```

Expected:

```text
Repository path is shown
Go language is detected
go.mod and README are included
Git status and diff are included when available
Sensitive/noisy paths are excluded
JSON output parses cleanly
```

Completed before Phase 7.
