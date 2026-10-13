---
type: spec
status: planned
tags: [feature, ui, workspace, config, security]
---

# Workspace settings screen

The **Settings** screen (sidebar footer): workspace-wide settings stored in `~/agents-ws/.foreman/settings.yaml` ([Workspace settings](../models/workspace-settings.md)) and tokens stored as [secrets](../models/secrets.md). Padding `48px 48px 64px`, max 760px. H1 "Settings"; sections labelled 13px `--tx4`, 36px apart; rows (grid `1fr auto`, padding `14px 0`, bottom border) with a title, an explanation and a control. Changes save immediately ([API](../architecture/api.md), `PUT /api/settings`).

## Workspace

| Row | Explanation | Control |
|---|---|---|
| Folder | Every subfolder with a .foreman/project.yaml is a project. Foreman doesn’t clone anything. It uses the git repos it finds in each project folder. | The path, read-only (Mono 13px `--tx2`) |
| Listen on | Anyone with the URL Foreman printed at start has full control. Keep this on your own machine unless you trust the network. | Segmented `This machine` / `Local network`. Choosing Local network asks for confirmation with a warning that anyone on the network can drive the agents ([Security](../architecture/security.md)). Takes effect on restart |
| Running sessions at once | Extra sessions wait in a queue until a slot frees up. | Stepper, 1–64, default 12 ([Agent runtime](../architecture/agent-runtime.md#concurrency)) |
| Show agent messages in terminals | ACP lines (`⇄`) in session panes. | Toggle, default on |

## Harnesses

"The command Foreman runs for each harness. It’s found on your PATH when you start the server."

Rows (grid `130px 1fr auto`): the harness label, a Mono command input (defaults `claude`, `codex`, `pi`, `hermes`), and the detection status: `found · <version>` (`--ok`) or `not set` / `not found` (`--err`). Custom harnesses are configured per agent and not listed here. See [Harness adapters](../architecture/harness-adapters.md).

## Forge accounts

"Tokens Foreman uses to push lead branches and open PRs. They stay on this machine. Each project's config names its forge."

One row each for **GitHub**, **Forgejo** and **Gitea**: the host and owner (or `—`), the token status (`token set` `--ok`, or `not connected` `--tx4`), and **`Connect`** / **`Change`**. Connect or Change opens an inline form: host URL (prefilled `github.com` for GitHub) and a write-only token field; Save validates the token against the forge and stores it ([Forge adapters](../architecture/forge-adapters.md), `PUT /api/secrets/forge/:kind`). A stored token is never shown again.

## Notifications

- **Browser notifications**: the permission state, `on` (`--ok`), `blocked` (`--err`), `turn on` (`--accTx`) or `n/a` (`--tx4`, unsupported browser), and a **`Request`** button shown only when permission can still be requested.
- Toggles (default on): `An agent needs me`, `An agent finishes`, `A PR is ready to merge` ([Notifications](./notifications.md)).

## Appearance

`Theme`: segmented `Dark` / `Light` ([Design system](../design/design-system.md)).

## Open questions

1. **System theme.** No "follow the system" option. Recommendation: add `System` as the default.
2. **Forge token form.** The prototype's buttons are inert, so the form above is new design. Confirm its shape (host + token, validated on save).
