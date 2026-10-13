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
| Show agent messages in terminals | team-message lines (`⇄`) in session panes. | Toggle, default on |

## Harnesses

"The command Foreman runs for each harness inside the agent image. Versions are what the image contains; see Maintenance."

Rows (grid `130px 1fr auto`): the harness label, a Mono command input (defaults `claude`, `codex`, `pi`, `hermes`), and the detection status in the harness image: `found · <version>` (`--ok`) or `not set` / `not found` (`--err`). The prototype detected them on the host PATH; with every session in a container, the image is what counts. Custom harnesses are configured per agent and not listed here. See [Harness adapters](../architecture/harness-adapters.md).

Each row also shows the login: `connected` (`--ok`), `reconnect needed` (`--err`) or `not connected` (`--tx4`), with **`Connect`** / **`Reconnect`**, which opens the login container's terminal in a dialog running the harness's own login (`claude /login` for Claude Code). See [Session runner → Credentials](../architecture/session-runner.md#credentials).

## Forge accounts

"Tokens Foreman uses to push lead branches, open PRs and watch them. They stay on this machine. Each project's config names its forge."

One row each for **GitHub**, **Forgejo** and **Gitea**: the host and owner (or `—`), the token status (`token set` `--ok`, or `not connected` `--tx4`), and **`Connect`** / **`Change`**. Connect or Change opens an inline form: host URL (prefilled `github.com` for GitHub) and a write-only token field; Save validates the token against the forge and stores it ([Forge adapters](../architecture/forge-adapters.md), `PUT /api/secrets/forge/:kind`). A stored token is never shown again.

## Notifications

- **Browser notifications**: the permission state, `on` (`--ok`), `blocked` (`--err`), `turn on` (`--accTx`) or `n/a` (`--tx4`, unsupported browser), and a **`Request`** button shown only when permission can still be requested.
- Toggles (default on): `An agent needs me`, `An agent finishes`, `A PR is ready to merge` ([Notifications](./notifications.md)).

## Maintenance

"The images agent sessions run in. Foreman builds them as a chain: base, harnesses, this workspace, each project. A layer without its own Containerfile simply passes the one above through."

At the top, Podman's state: `Podman 5.x · running` (`--ok`), or on macOS the machine's state (`machine running` / `starting…` / `stopped`), from [Session runner → The Podman service](../architecture/session-runner.md#the-podman-service).

Then one row per image of the chain ([Agent images](../architecture/tech-stack.md#agent-images)), indented to show it: **Base**, **Harnesses**, **Workspace**, then one row per project. Each row shows:

- the image name (Mono, e.g. `localhost/foreman/ws/agents-ws/ledger`) and its source: `built in`, the Containerfile path, or `transparent` (`--tx4`)
- the state: `ready · built <relative time>` (`--ok`), `building…` (`--team`), `failed` (`--err`) or `not built yet` (`--tx4`)
- on the Harnesses row, the versions inside (`claude 2.1.4 · codex 0.48.0 · …`)
- **`Refresh`** (Base and Harnesses): rebuild with the latest base image and harness CLIs, then rebuild everything below, with an option "Also prune outdated images". Running sessions keep their image; new and restarted sessions use the refreshed one.
- A transparent row shows where its Containerfile would go, so the CEO knows where to create one.
- **`Log`**: the last build log, in a read-only Mono panel.

Below the list: **`Prune`**, with the space it would reclaim. It removes Foreman's outdated images and build cache only, never other images ([Agent images](../architecture/tech-stack.md#agent-images)).

A failed build also raises a **blocked** [request](./requests.md), so it reaches Home.

## Appearance

`Theme`: segmented `System` / `Dark` / `Light`, default `System` (follows the operating system's `prefers-color-scheme`, live). Stored in [UI state](../models/ui-state.md) ([Design system](../design/design-system.md)).
