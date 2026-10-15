---
type: spec
status: planned
tags: [feature, ui, workspace, config, security]
---

# Workspace settings screen

The **Settings** screen (sidebar footer): workspace-wide settings stored in `~/agents-ws/.foreman/settings.toml` ([Workspace settings](../models/workspace-settings.md)) and tokens stored as [secrets](../models/secrets.md). Padding `48px 48px 64px`, max 760px. H1 "Settings"; sections labelled 13px `--tx4`, 36px apart; rows (grid `1fr auto`, padding `14px 0`, bottom border) with a title, an explanation and a control. Changes save immediately ([API](../architecture/api.md), `PUT /api/settings`).

## Workspace

| Row | Explanation | Control |
|---|---|---|
| Folder | Every subfolder with a project/project.toml is a project. Foreman doesn’t clone anything. It uses the git repos it finds in each project folder. | The path, read-only (Mono 13px `--tx2`) |
| Listen on | Anyone with the URL Foreman printed at start has full control. Keep this on your own machine unless you trust the network. | Segmented `This machine` / `Local network`. Choosing Local network asks for confirmation with a warning that anyone on the network can drive the agents ([Security](../architecture/security.md)). Takes effect on restart |
| Running sessions at once | Extra sessions wait in a queue until a slot frees up. | Stepper, 1–64, default 12 ([Agent runtime](../architecture/agent-runtime.md#concurrency)) |
| Run agents in | Containers isolate each agent from your machine. Without them, agents run as plain processes with full access to it. | Segmented `Containers (Podman)` / `Directly (no isolation)`. Choosing Directly asks for confirmation; while agents run directly, a warning stays visible here. **Takes effect when Foreman restarts** (decided 2026-10-15): until then the row says `Restart Foreman to switch` and Maintenance shows the engine still running. `Containers` can't be chosen while Podman isn't installed (on macOS: no machine), and the row says why: Foreman wouldn't start with it ([Workspace settings](../models/workspace-settings.md)) |
| Show agent messages beside terminals | The marker strip (`⇄` team traffic, escalations, notes) along each session pane. The Messages drawer always lists the traffic. | Toggle, default on |

## Harnesses

"The coding programs agents run, built into the agent image. Each needs its own login, once per workspace."

Rows (grid `130px 1fr auto`): the harness label, its command in Mono `--tx3` (read-only: Foreman builds the image, so the command is always the harness's own, `claude`, `codex`, `pi`, `hermes`; decided 2026-10-15, no command setting), and the detection status in the harness image: `found · <version>` (`--ok`) or `not found` (`--err`). Custom harnesses are configured per agent and not listed here. See [Harness adapters](../architecture/harness-adapters.md).

**Phasing:** the detection status comes with the [full image chain](../../backlog/image-chain.md); until then a row shows the command and the login only.

Each row also shows the login: `connected` (`--ok`), `reconnect needed` (`--err`) or `not connected` (`--tx4`), with **`Connect`** / **`Reconnect`**, which opens the **Connect dialog** (`ConnectDialog`, 760px): the login container's terminal running the harness's own login (for Claude Code: Claude Code itself, where the CEO types `/login`, since it never asks on its own). Under the terminal, the login state, followed while open (`Not connected yet.` in `--tx4`, then `<Harness> is connected.` in `--ok`), and one button: `Cancel`, which becomes a primary `Done` once connected. It closes only through that button, never Escape or the backdrop: Escape belongs to the terminal, and a stray click mustn't end the login. Closing ends the login container (`DELETE /api/harnesses/:id/connect`), logged in or not. The same dialog opens from [Deploy a team](./deploy-team.md). See [Session runner → Credentials](../architecture/session-runner.md#credentials). **Phasing:** `reconnect needed` needs the adapter to detect an expired login; until then a row shows `connected` or `not connected`.

## Forge accounts

"Tokens Foreman uses when you ask it to create a PR or sync a repo, and to watch those PRs. They stay on this machine. Each project's config names its forge."

One row each for **GitHub**, **Forgejo** and **Gitea**: the host and owner (or `—`), the token status (`token set` `--ok`, or `not connected` `--tx4`), and **`Connect`** / **`Change`**. Connect or Change opens an inline form: host URL (prefilled `github.com` for GitHub) and a write-only token field; Save validates the token against the forge and stores it ([Forge adapters](../architecture/forge-adapters.md), `PUT /api/secrets/forge/:kind`). A stored token is never shown again.

Below the rows, **Workspace repo**: where the [workspace repo](../models/workspace.md#the-workspace-repo) is hosted, so the Chief of Staff's team changes can become PRs there (Create PR on their merge request): `No forge` (the default; the CEO merges them locally) or one of the connected forges, with the repo's URL (`workspace_forge` in [Workspace settings](../models/workspace-settings.md#schema)). With a forge set, a small outline **`Sync`** button fetches the forge's `main`, merges it in and pushes the workspace repo's `main` there, stopping with a message on a conflict; nothing is pushed automatically ([Forge adapters → Sync](../architecture/forge-adapters.md#sync-the-reserved-repos)). **Phasing:** Sync comes with the [Git epic](../../backlog/epics/git.md); until then the button shows, disabled.

## Notifications

- **Browser notifications**: the permission state (raised only while the tab is hidden: when the CEO is looking, the toast says it already), `on` (`--ok`), `blocked` (`--err`), `turn on` (`--accTx`) or `n/a` (`--tx4`, unsupported browser), and a **`Request`** button shown only when permission can still be requested.
- Toggles (default on): `An agent needs me`, `An agent finishes`, `A merge is requested` ([Notifications](./notifications.md)).

## Maintenance

**Phasing:** phase 1 has one image for every project ([Agent images](../architecture/tech-stack.md#agent-images)), so the section shows Podman's state and that one image: its state, its last build log, and **`Rebuild`** (without cache, picking up the latest Claude Code). Phase 1 shows the state without the build time, and no harness versions yet (neither here nor on the Harnesses rows). The chain rows, Refresh with prune, Customize hints and Prune below are the target, built with the [full image chain](../../backlog/image-chain.md).

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
