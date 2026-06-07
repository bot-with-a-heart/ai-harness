# Troubleshooting

## Run Doctor First

```powershell
ai-harness config doctor
```

Doctor checks config readability, LM Studio reachability, Codex executable discovery, and history directory writability.

## LM Studio Is Not Reachable

Check:

```text
LM Studio local server is running
base_url points to the right host and port
firewall allows local access
model is downloaded and loadable
```

Then run:

```powershell
ai-harness models list --timeout 30s
```

## Codex Is Not Found

Confirm Codex is installed and available on PATH:

```powershell
codex --version
```

Then run:

```powershell
ai-harness config doctor
```

## Classification Falls Back To Heuristic

This usually means the local classifier request failed or returned invalid JSON.

Use:

```powershell
ai-harness classify --heuristic --summary "Your task"
ai-harness --log-level debug classify "Your task"
```

## Patch Was Not Applied

Safe edit workflows require explicit approval:

```text
yes
```

Any other answer leaves the workspace unchanged.

Patch application can also fail if generated diff paths do not match the current working tree.

## Tests Are Skipped

`--test-command auto` detects:

```text
go.mod -> go test ./...
package.json -> npm test
pyproject.toml -> pytest
```

Pass an explicit command when auto detection is not enough:

```powershell
ai-harness run --edit --test-command "go test ./..." "Add tests"
```

## History Is Empty

History is stored next to the active config. If you use `--config`, pass the same path to history commands:

```powershell
ai-harness history --config <config-path> list
```

## Encrypted History Is Locked

For the passphrase provider, provide the passphrase to security commands or set the environment variable used by normal history reads:

```powershell
$env:AI_HARNESS_PASSPHRASE = "<passphrase>"
ai-harness security verify
ai-harness history list
```

If `security verify` reports invalid records, the passphrase may be wrong or the encrypted files may be corrupt. Verification exits with an error when invalid encrypted records are found.

If you exported recovery material, you can verify or rotate with it:

```powershell
ai-harness security verify --recovery-file recovery.json
ai-harness security rotate-key --recovery-file recovery.json --new-passphrase <new-passphrase>
```

## Migrate Existing Plaintext History

After initializing security, migrate old plaintext records:

```powershell
ai-harness security migrate --passphrase <passphrase>
ai-harness security verify --passphrase <passphrase>
```

Migration removes plaintext history files only after each record is encrypted and verified.

## Obsidian Export Does Nothing

The Obsidian integration is disabled by default.

Configure a vault or pass one explicitly:

```powershell
ai-harness memory obsidian init --vault <vault-path>
ai-harness memory obsidian export --vault <vault-path> --dry-run
```

If export reports a conflict, the target note exists but is not marked as AI Harness managed. Review the note and re-run with `--force` only when replacing it is intentional.

## Need Integration-Friendly Output

Use JSON flags where available and keep logs on stderr:

```powershell
ai-harness security status --json
ai-harness history list --json
ai-harness history show <id> --json
ai-harness --log-level disabled classify --heuristic "Task"
```
