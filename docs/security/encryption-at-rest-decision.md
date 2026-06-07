# Encryption-at-Rest Decision Record

Status: Approved and implemented in Phase 13  
Phase: 12  
Date: 2026-06-06

## Decision Summary

AI Harness will implement encryption at rest for sensitive local-first data stores while preserving CLI/plugin compatibility and optional human-readable Obsidian exports.

User decisions captured for this draft:

```text
Threat model: compliance-grade protection for team/customer data
Default mode: encryption on by default for sensitive stores, user can disable
Encrypted stores: history, memory, logs
Retention: configurable per data type
Key management: OS keychain first, passphrase fallback
Recovery: recovery export plus key rotation required
Obsidian: sensitive content allowed only after explicit flag/approval
Compliance-required mode: included in the first encryption implementation
Model catalog encryption: optional/deferred
Encryption primitive: Go standard-library AES-GCM for the first implementation
Repo context snapshots: summary-only by default; full snapshots opt-in and encrypted
```

This decision record guided the Phase 13 implementation. Phase 13 implemented the first concrete encrypted store for history records and added the key-management, migration, verification, recovery, and rotation commands needed for future sensitive stores.

## Phase 13 Implementation Status

Implemented:

```text
AES-256-GCM encrypted history envelopes
per-record random nonce and authentication
OS keychain key provider
passphrase-derived key provider using scrypt
security-required config mode
plaintext history migration to .json.enc files
security status and status --json
security verify with failing exit on invalid encrypted records
explicit recovery export
recovery-file verification, unlock, and rotation
key rotation for encrypted history records
summary-only repo context retention by default
full repo context snapshot retention disabled by default
```

Prepared for future stores:

```text
config flags for encrypted memory and logs
shared security package for additional encrypted stores
retention policy for encrypted full repo snapshots if later enabled
```

## Security Posture

AI Harness should be compliance-ready for local-first use, but this does not by itself claim certification under a specific framework.

The product should protect against:

```text
lost or stolen laptops
cloud backup or sync exposure
accidental file sharing
casual local snooping
team/customer data exposure from plaintext local stores
```

The product should reduce but cannot fully prevent exposure from:

```text
active malware running as the user
admin-level compromise after unlock
screen capture
terminal scrollback
user-approved plaintext export
decrypted data intentionally passed to plugins or external tools
```

Documentation and command output must be clear about these boundaries.

## Data Classification

### Restricted

Must be encrypted at rest when stored by AI Harness:

```text
history records
long-term memory records
execution logs
prompts and model responses, when retained
generated diffs and patches, when retained
repo context snapshots, when retained
backup files containing encrypted-store data
recovery material metadata, excluding the recovery secret itself
```

Secrets must not be newly stored as plaintext in config files. Phase 13 should support OS keychain storage and/or environment-variable references for sensitive provider credentials.

### Confidential

May remain plaintext only if explicitly configured or redacted:

```text
Obsidian notes
human-readable summaries
plugin-facing summaries
diagnostic bundles
```

Sensitive Obsidian exports require an explicit flag or approval at export time.

### Operational

May remain plaintext by default:

```text
model catalog data
non-secret configuration
documentation
binary release artifacts
non-sensitive status output
```

The model catalog is considered non-sensitive operational metadata unless a future user setting marks it sensitive.

Model catalog encryption is optional/deferred for Phase 13. The implementation may support encrypting it later, but it is not required for the initial encryption-at-rest release.

## Storage Locations

Sensitive stores currently or expected to exist:

```text
~/.ai-harness/history/
~/.ai-harness/logs/
~/.ai-harness/memory/
~/.ai-harness/backups/
```

Config remains:

```text
~/.ai-harness/config.toml
```

Config may contain security policy and key references, but should not contain encryption keys or sensitive provider secrets.

Optional Obsidian exports remain in the user-selected vault and are not the canonical store.

## Encryption Requirements

Phase 13 should implement:

```text
authenticated encryption for selected stores
per-record or per-file integrity checks
clear encrypted file format/version marker
safe failure on corrupt or tampered records
no silent plaintext duplicate records
structured locked/unlocked status for plugins
```

Selected default for the first implementation:

```text
AES-256-GCM using Go standard-library cryptography
random nonce per encrypted object
store metadata needed for migration and verification
```

XChaCha20-Poly1305 may be reconsidered later if nonce-management or streaming requirements justify another dependency.

## Default Policy

Encryption is on by default for sensitive stores.

Users may disable encryption for local experimentation, but disabling must require an explicit command or config change and must show a clear warning.

Compliance-oriented deployments should be able to enforce encryption-required mode later. In encryption-required mode, disabling encryption must be blocked.

Phase 13 should include encryption-required mode immediately, even if the first version exposes it as a local policy flag rather than a full enterprise policy manager.

## Retention Policy

Retention is configurable per data type.

Default retention should be conservative:

```text
history: retain metadata and summaries
memory: retain curated records
logs: retain bounded diagnostic logs
prompts/responses: do not retain verbatim unless enabled
generated diffs/patches: do not retain verbatim unless enabled
repo context snapshots: do not retain by default
```

When verbatim prompts, responses, diffs, patches, or context snapshots are retained, they are Restricted data and must be encrypted.

Repository context retention policy:

```text
summary-only by default
no full file-content snapshot retention by default
no full git diff retention by default
full repo context snapshots require explicit opt-in
opted-in full snapshots must be encrypted
```

Summary records may include operational metadata such as languages, frameworks, selected provider, files touched, tests run, and high-level context-source counts.

## Key Management

Primary key storage:

```text
OS keychain first
```

Examples:

```text
Windows Credential Manager
macOS Keychain
Linux Secret Service/libsecret when available
```

Fallback:

```text
passphrase-derived key
```

Passphrase fallback must use a modern password-based key derivation function with strong parameters and a per-install salt.

File-based raw keys are not preferred and should be avoided unless a future deployment scenario explicitly requires them.

## Recovery And Rotation

Phase 13 must support both:

```text
recovery export
key rotation
```

Recovery export should produce user-held recovery material that can unlock or re-wrap the data encryption key. The recovery material must be shown or written only through an explicit command and must never be logged.

If all key and recovery material is lost:

```text
encrypted data is unrecoverable
```

The CLI must warn users before enabling encryption without recovery material.

Key rotation must re-wrap or migrate encrypted stores without changing unrelated plaintext config.

## Backups

Backups of encrypted stores must remain encrypted.

Backup commands must avoid creating plaintext temporary copies. If temporary files are unavoidable, they must be written inside the protected harness directory, cleaned up, and documented.

Backup metadata may remain plaintext only when it does not reveal sensitive task or repo content.

## Plugin Access Rules

Plugins and editor integrations must not receive broad decrypted stores by default.

Default plugin access:

```text
security status
record existence
redacted summaries
scoped records requested by command
```

Decrypted sensitive data may be passed to a plugin only when:

```text
the store is unlocked
the command explicitly requests that data
the plugin scope permits it
the output format is structured and auditable
```

Future plugin manifests should declare requested security scopes.

## Obsidian Export Rules

Default Obsidian export:

```text
redacted summaries only
model catalog allowed
recent task outcome summaries allowed only with --include-history
task text redacted unless --include-task-text is provided
```

Sensitive content may be exported to Obsidian only after explicit flag/approval.

Obsidian export files remain Markdown by default so they are useful in Obsidian. Encrypted Obsidian files are not the default because they reduce Obsidian's value as a readable vault.

If the user enables sensitive Obsidian exports, the CLI must warn that the exported Markdown is plaintext inside the vault unless a future encrypted-export mode is explicitly selected.

## Local-First Sync

Local sync and cloud backup are allowed only if encrypted stores remain encrypted.

Plaintext Obsidian exports are user-approved copies and must be treated separately from canonical encrypted stores.

Sync should not require a cloud account or remote service.

## Migration Requirements

Phase 13 must provide migration from existing plaintext stores.

Migration requirements:

```text
detect plaintext history records
encrypt records in place or into a new encrypted store
avoid leaving plaintext duplicates
create a backup plan before migration
verify migrated record counts
fail safely if migration is interrupted
```

Existing plaintext records should not be silently deleted until migration verification succeeds.

## Proposed Phase 13 Commands

```powershell
.\ai-harness.exe security init
.\ai-harness.exe security status
.\ai-harness.exe security lock
.\ai-harness.exe security unlock
.\ai-harness.exe security rotate-key
.\ai-harness.exe security export-recovery
.\ai-harness.exe security verify
.\ai-harness.exe security migrate
```

`security status --json` should provide machine-readable state for plugins.

## Manual Testing Requirements For Phase 13

Verify:

```powershell
.\ai-harness.exe security init
.\ai-harness.exe security status
.\ai-harness.exe security status --json
.\ai-harness.exe security verify
.\ai-harness.exe security export-recovery
.\ai-harness.exe security rotate-key
.\ai-harness.exe security migrate
.\ai-harness.exe history list
.\ai-harness.exe memory obsidian export --dry-run
```

Expected:

```text
new sensitive records are encrypted at rest
plaintext records migrate without data loss
locked stores fail with clear guidance
unrelated commands still work while locked
corrupt records fail safely
keys and recovery material never appear in logs
plugins receive redacted or scoped data by default
Obsidian exports require explicit approval for sensitive content
```

## User Decisions Resolved Before Phase 13

```text
Compliance-required mode: include immediately
Model catalog encryption: optional/deferred
First implementation primitive: Go standard-library AES-GCM
Repo context snapshots: summary-only by default; full snapshots opt-in and encrypted
```

## Approval

Phase 12 was completed by user approval before Phase 13 implementation.
