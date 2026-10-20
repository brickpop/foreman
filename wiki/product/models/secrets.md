---
type: spec
status: planned
tags: [model, security, forge]
---

# Secrets

Credentials Foreman uses on the CEO's behalf: in v1, the forge tokens used, on the CEO's Create PR or Sync, to push branches, open, watch and close PRs, and post comments ([Forge adapters](../architecture/forge-adapters.md)). Set from the [Settings screen](../features/settings.md) through write-only fields (`PUT /api/secrets/forge/:kind`, see [API](../architecture/api.md)): the UI can show whether a token is set, never its value. Harness logins are not secrets in this store: each lives in its profile's folder, `~/.foreman/profiles/<name>/` (0600), written only by the login container ([Session runner → Credentials](../architecture/session-runner.md#credentials)).

## Storage

A file, `~/.foreman/secrets`, with 0600 permissions: simplest, and it works on headless Linux without a desktop keyring. It sits behind a small secret-store interface, so an OS keychain backend (macOS Keychain, Secret Service) can be added later without touching its users. With Podman the file is safe from agents, since it is never mounted.

| Key | Value |
|---|---|
| `forge/github` | Token for the GitHub account |
| `forge/forgejo` | Token for the Forgejo account |
| `forge/gitea` | Token for the Gitea account |

## Invariants

- Secrets are never written into a repo (code, project or global), a config file or state.db.
- **Logs, the one exception Foreman can't fully prevent:** a harness can print a secret into its PTY (an env dump, a token in an error), and session logs are raw bytes. Foreman redacts, on write, the exact values it holds or injected into the container; anything else a harness prints is logged as is. This limitation is documented in [Security](../architecture/security.md); log retention is in [Persistence](../architecture/persistence.md).
- Agents never receive forge tokens. Pushes to the forge are done by Foreman, not the agents.
- The secrets file (and the rest of Foreman's [home](./home.md), but an agent's own sandbox and project repo) is never mounted into an agent container ([Session runner](../architecture/session-runner.md)). Containers also don't get the CEO's home directory, so `~/.config/gh` and similar credentials stay out of reach.
- There is no read endpoint for secret values.
