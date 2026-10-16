---
type: task
status: todo
priority: medium
tags: [feature, decision]
epic: install-first-run
---

# Harness logins shared across workspaces, with profiles

**Why:** the CEO, 2026-10-16. Logins live in each workspace (`<workspace>/.foreman/credentials/<harness>/`), so every new workspace logs in again ([Session runner → Credentials](../product/architecture/session-runner.md#credentials)). The CEO may keep projects in several workspaces (personal, professional, work, experiments) and wants one place for logins, maybe with more than one account.

**Proposed** (to confirm with the CEO before building):
- **One store per user**, outside any workspace: `<user config folder>/foreman/credentials/<profile>/<harness>/` (on macOS, `~/Library/Application Support/foreman/…`).
- **Profiles:** `default` always exists, and the CEO can add more (`work`, `personal`). A workspace setting names its profile, `default` unless set. Someone with one account never sees profiles.
- **Settings → Harnesses** shows the workspace's profile and its logins. **Connect** logs in to that profile.
- **Migration:** a workspace's existing login moves into `default` when it has none yet, or into a profile named after the workspace.
- **Development:** `.dev/*` workspaces and the integration tests use the `default` profile, so no more logging in per workspace.

**Edge cases:**
- **A token refresh** by one agent rewrites the shared login that agents of other workspaces use at the same moment. Today that only happens within one workspace. To check with Claude Code's refresh.
- **Podman on macOS** must still see the folder: the user's home is shared with the machine, so the config folder is.
- **Per team or per project profiles** are possible later. Per workspace is the simplest fit for "personal / work" folders.

- [ ] the CEO confirms (store location, profiles per workspace)
- [ ] the store, the setting, Settings → Harnesses, migration; tests (two workspaces share a login; profiles stay apart)
