# Roadmap

AI Harness is currently stopped for review after Phase 13.

## Completed or Active

```text
Phase 0 - Project bootstrap
Phase 1 - Configuration system
Phase 2 - LM Studio provider and model catalog
Phase 3 - Codex provider
Phase 4 - Task classification
Phase 5 - Unified routing
Phase 6 - Repository context
Phase 7 - Safe edit workflow
Phase 8 - Escalation workflow
Phase 9 - History and audit trail
Phase 10 - Production readiness
Phase 11 - Optional Obsidian vault memory integration
Phase 12 - Encryption-at-rest decision interview
Phase 13 - Encryption-at-rest implementation
```

## Planned

```text
Future phases are intentionally deferred until Phase 13 review is complete.
```

Phase 11 is optional by design. Obsidian must enhance the local-first memory experience without becoming a hard dependency for the core harness.

Phase 12 was a collaborative planning phase. It interviewed the user, clarified the threat model, and recorded the encryption-at-rest decision for memory, logs, history, catalogs, and optional Obsidian exports.

Phase 13 implemented the first encryption-at-rest release: encrypted history records, OS keychain and passphrase key providers, recovery export, key rotation, migration, verification, and machine-readable security status.
