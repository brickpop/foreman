---
type: spec
status: planned
tags: [model, security, forge]
---

# Secrets

Credentials Foreman uses on the CEO's behalf: in v1, the forge tokens used to push lead branches, open and merge PRs, and post comments ([Forge adapters](../architecture/forge-adapters.md)). Set from the [Workspace settings screen](../features/settings.md) through write-only fields (`PUT /api/secrets/forge/:kind`, see [API](../architecture/api.md)): the UI can show whether a token is set, never its value.

## Storage

The PRD allows either the OS keychain, or `~/agents-ws/.foreman/secrets` with 0600 permissions. Which one is an open question.

| Key | Value |
|---|---|
| `forge/github` | Token for the GitHub account |
| `forge/forgejo` | Token for the Forgejo account |
| `forge/gitea` | Token for the Gitea account |

## Invariants

- Secrets are never written into project folders, config files, state.db or logs.
- Agents never receive forge tokens. Pushes to the forge are done by Foreman, not the agents.
- The secrets file (and the whole workspace `.foreman/`) is never mounted into an agent container ([Session runner](../architecture/session-runner.md)). Containers also don't get the CEO's home directory, so `~/.config/gh` and similar credentials stay out of reach.
- There is no read endpoint for secret values.

## Open questions

1. **Keychain or file.** Options: OS keychain (macOS Keychain, Secret Service on Linux), or a 0600 file. Recommendation: the 0600 file in v1 (simplest, works on headless Linux without a desktop keyring), behind a small secret-store interface so a keychain backend can be added later. With Podman, the file is safe from agents as long as it is never mounted.
2. **Secrets in session logs.** A harness can print a secret (an env dump, a token in an error, its own model credential) into its PTY, which is logged raw, so "secrets are never written to logs" can't be guaranteed. Options: accept and document it; redact known secret values from the stream before logging. Recommendation: redact the values Foreman itself holds or injected (exact matches) on write, and document the rest as a limitation ([Security](../architecture/security.md)). Retention is in [Persistence](../architecture/persistence.md#open-questions).
