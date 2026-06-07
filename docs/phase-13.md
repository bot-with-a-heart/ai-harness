# Phase 13 Encryption-at-Rest Implementation

Phase 13 implements the decision made in Phase 12.

## Objective

Implement encryption-at-rest for the selected AI Harness data stores while preserving the local-first product direction and plugin compatibility.

## Scope

Implemented scope:

```text
encrypted history records
OS keychain key storage
passphrase-derived key fallback
security-required mode
plaintext history migration
encrypted record verification
recovery export
key rotation
plugin-friendly security status JSON
summary-only repo context retention by default
```

Prepared for later:

```text
canonical memory store encryption
persistent execution log encryption
encrypted full repo context snapshots if explicitly enabled later
model catalog encryption if a future user setting marks it sensitive
```

## Commands

```powershell
.\ai-harness.exe security init --provider os-keychain --required
.\ai-harness.exe security init --provider passphrase --passphrase <passphrase> --required
.\ai-harness.exe security status
.\ai-harness.exe security status --json
.\ai-harness.exe security migrate --passphrase <passphrase>
.\ai-harness.exe security verify --passphrase <passphrase>
.\ai-harness.exe security export-recovery --passphrase <passphrase> --output recovery.json
.\ai-harness.exe security rotate-key --passphrase <old-passphrase> --new-passphrase <new-passphrase>
.\ai-harness.exe security rotate-key --recovery-file recovery.json --new-passphrase <new-passphrase>
.\ai-harness.exe security lock
.\ai-harness.exe security unlock --passphrase <passphrase>
.\ai-harness.exe security unlock --recovery-file recovery.json
```

Passphrase provider commands can read the passphrase from `AI_HARNESS_PASSPHRASE` when `--passphrase` is omitted.

## Implemented Behavior

```text
new history records are written as .json.enc after security init
existing plaintext .json history records migrate to encrypted records
verification fails safely on wrong keys, corrupt records, or tampering
key material is not written to config or logs
recovery material is written only by explicit command
recovery material can verify, unlock, or rotate current encrypted history
rotation re-encrypts existing encrypted history records
status --json exposes structured state for editor integrations
history commands use encrypted storage automatically after security init
```

## Repository Context Policy

```text
summary-only by default
full repo context snapshot retention is disabled by default
if full snapshots are later enabled, they must be encrypted
```

## Safety Requirements

Verified:

```text
plaintext records are not silently duplicated
keys are not written into logs
wrong passphrases fail verification
corrupt encrypted records fail safely
locked passphrase stores return clear guidance
plugin-facing status remains structured
Obsidian exports still require explicit flags for history and task text
```

## Manual Testing

Verified with a temporary config:

```powershell
.\ai-harness.exe config --path <temp-config> init
.\ai-harness.exe classify --config <temp-config> --heuristic --summary "Phase 13 smoke"
.\ai-harness.exe security init --config <temp-config> --provider passphrase --passphrase <passphrase> --required
.\ai-harness.exe security status --config <temp-config> --json
.\ai-harness.exe security migrate --config <temp-config> --passphrase <passphrase>
.\ai-harness.exe security verify --config <temp-config> --passphrase <passphrase>
.\ai-harness.exe security export-recovery --config <temp-config> --passphrase <passphrase> --output <recovery-file>
.\ai-harness.exe security unlock --config <temp-config> --recovery-file <recovery-file>
.\ai-harness.exe security rotate-key --config <temp-config> --recovery-file <recovery-file> --new-passphrase <new-passphrase>
.\ai-harness.exe security verify --config <temp-config> --passphrase <new-passphrase>
.\ai-harness.exe history --config <temp-config> list
```

Expected:

```text
history is readable with the available key
plaintext history files are removed after verified migration
encrypted history files remain as .json.enc
old passphrase fails after rotation
old recovery material fails after rotation
recovery export is explicit and chmod-style restricted where supported
```

## Completion

Completed before review after Phase 13.
