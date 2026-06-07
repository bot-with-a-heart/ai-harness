# Phase 12 Encryption-at-Rest Decision Interview

Phase 12 was a collaborative decision phase.

## Objective

Interview the user and decide how AI Harness should handle encryption at rest for memory, logs, history, model catalog data, and optional Obsidian exports.

This phase did not implement encryption.

## Decision Areas

Discuss and decide:

```text
what data must be encrypted
what data can remain plaintext
whether secrets are stored at all
whether redaction is enough for some records
whether encryption is always on or opt-in
whether keys use passphrases, OS keychain storage, hardware-backed storage, or file-based keys
how recovery and key rotation should work
how backups should work
how plugins should access encrypted data
how Obsidian exports should be handled
how local-first sync should work without weakening security
```

## Interview Questions

At minimum, ask:

```text
What are we protecting against: stolen laptop, malware, cloud sync exposure, accidental sharing, or all of these?
Should execution logs store prompts and model responses verbatim?
Should generated patches and diffs be retained?
Should repo context snapshots be retained?
Should model catalog data be encrypted or considered non-sensitive?
Should Obsidian notes include sensitive memory, summaries only, or no sensitive data?
Should encryption be mandatory for all users or configurable?
What is acceptable if a key is lost?
Should plugins receive decrypted data, summaries, or scoped records only?
```

## Deliverable

Create a written decision record, for example:

```text
docs/security/encryption-at-rest-decision.md
```

The decision record should define:

```text
data classification
storage locations
encryption requirements
key management approach
backup and recovery expectations
plugin access rules
migration requirements
manual testing requirements for Phase 13
```

## Completion Criteria

Completed before Phase 13. The user approved summary-only repository context retention by default, encrypted full snapshots only when explicitly enabled, compliance-required mode, OS keychain first with passphrase fallback, recovery export, and key rotation.
