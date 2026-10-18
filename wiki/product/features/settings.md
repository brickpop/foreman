---
type: spec
status: planned
tags: [feature, ui, workspace, config, security]
---

# Settings screen

The **Settings** screen (sidebar footer), in two tabs (the CEO, 2026-10-18), `Workspace | Global` (`/settings?tab=global`; the same tab bar as a project's). **Workspace**: this workspace's settings, stored in `~/agents-ws/.foreman/settings.toml` ([Workspace settings](../models/workspace-settings.md)), and tokens stored as [secrets](../models/secrets.md): Workspace, Forge accounts and Notifications. **Global**: the CEO's, on this machine, the same in every workspace: [Profiles](#profiles), [Maintenance](#maintenance) (one Podman and one agent image for every workspace; the CEO, 2026-10-18) and [Appearance](#appearance). Padding `48px 48px 64px`, max 760px. H1 "Settings"; sections labelled 13px `--tx4`, 36px apart; rows (grid `1fr auto`, padding `14px 0`, bottom border) with a title, an explanation and a control. Changes save immediately ([API](../architecture/api.md), `PUT /api/settings`).

## Workspace

| Row | Explanation | Control |
|---|---|---|
| Folder | Every subfolder with a project/project.toml is a project. Foreman doesn’t clone anything. It uses the git repos it finds in each project folder. | The path, read-only (Mono 13px `--tx2`) |
| Listen on | Anyone with the URL Foreman printed at start has full control. Keep this on your own machine unless you trust the network. | Segmented `This machine` / `Local network`. Choosing Local network asks for confirmation with a warning that anyone on the network can drive the agents ([Security](../architecture/security.md)). Takes effect on restart |
| Running sessions at once | Extra sessions wait in a queue until a slot frees up. | Stepper, 1–64, default 12 ([Agent runtime](../architecture/agent-runtime.md#concurrency)) |
| Run agents in | Containers isolate each agent from your machine. Without them, agents run as plain processes with full access to it. | Segmented `Containers (Podman)` / `Directly (no isolation)`. Choosing Directly asks for confirmation; while agents run directly, a warning stays visible here. **Takes effect when Foreman restarts** (decided 2026-10-15): until then the row says `Restart Foreman to switch` and Maintenance shows the engine still running. `Containers` can't be chosen while Podman isn't installed (on macOS: no machine), and the row says why: Foreman wouldn't start with it ([Workspace settings](../models/workspace-settings.md)) |

## Profiles

On the **Global** tab. "Each profile is a harness and its login, yours on this machine: every workspace's projects can use it. A project uses default unless its settings pick another." See [Session runner → Credentials](../architecture/session-runner.md#credentials).

Rows (grid `1fr auto auto`): the profile's name (Mono 600) and its harness label (13px `--tx4`); its state, `connected` (`--ok`) or `not connected` (`--tx4`); **`Connect`** / **`Reconnect`**, and **`Delete`** on every profile but `default`. Delete asks first: "Delete the <name> profile? Its login goes with it. Projects that use it, in any workspace, can't start agents until they pick another profile." No rename for now: other workspaces' projects name it.

Under the rows, **`New profile`** turns into a form: `Name` (Mono, placeholder `work`; what is typed is made into a profile name, shown beside it when it differs: `My Work` → `my-work`), `Harness` (a select of the supported harnesses, Claude Code only today), `Cancel` and `Create`. Creating opens the profile's Connect dialog straight away, since a profile without a login can't run agents. `GET /api/profiles`, `POST /api/profiles`, `DELETE /api/profiles/:name` ([API](../architecture/api.md)).

**`Connect`** / **`Reconnect`** opens the **Connect dialog** (`ConnectDialog`, 760px, title `Connect <profile>`): the login container's terminal running the profile's harness login (for Claude Code: Claude Code itself, where the CEO types `/login`, since it never asks on its own). Under the terminal, the login state, followed while open (`Not connected yet.` in `--tx4`, then `<profile> is connected.` in `--ok`), and one button: `Cancel`, which becomes a primary `Done` once connected. It closes only through that button, never Escape or the backdrop: Escape belongs to the terminal, and a stray click mustn't end the login. Closing ends the login container (`DELETE /api/profiles/:name/connect`), logged in or not. The same dialog opens from [Deploy a team](./deploy-team.md) and the setup checklist ([First run](./first-run.md)). **Phasing:** `reconnect needed` (an expired login) needs the adapter to detect it, and each harness's detection status in the agent image (`found · <version>`) comes with the [full image chain](../../backlog/image-chain.md).

## Forge accounts

"Tokens Foreman uses when you ask it to create a PR or sync a repo, and to watch those PRs. They stay on this machine. Each project's config names its forge."

One row each for **GitHub**, **Forgejo** and **Gitea**: the host and owner (or `—`), the token status (`token set` `--ok`, or `not connected` `--tx4`), and **`Connect`** / **`Change`**. Connect or Change opens an inline form: host URL (prefilled `github.com` for GitHub) and a write-only token field; Save validates the token against the forge and stores it ([Forge adapters](../architecture/forge-adapters.md), `PUT /api/secrets/forge/:kind`). A stored token is never shown again.

Below the rows, **Workspace repo**: where the [workspace repo](../models/workspace.md#the-workspace-repo) is hosted, so the Chief of Staff's team changes can become PRs there (Create PR on their merge request): `No forge` (the default; the CEO merges them locally) or one of the connected forges, with the repo's URL (`workspace_forge` in [Workspace settings](../models/workspace-settings.md#schema)). With a forge set, a small outline **`Sync`** button fetches the forge's `main`, merges it in and pushes the workspace repo's `main` there, stopping with a message on a conflict; nothing is pushed automatically ([Forge adapters → Sync](../architecture/forge-adapters.md#sync-the-reserved-repos)). **Phasing:** Sync comes with the [Git epic](../../backlog/epics/git.md); until then the button shows, disabled.

## Notifications

- **Browser notifications**: the permission state (raised only while the tab is hidden: when the CEO is looking, the toast says it already), `on` (`--ok`), `blocked` (`--err`), `turn on` (`--accTx`) or `n/a` (`--tx4`, unsupported browser), and a **`Request`** button shown only when permission can still be requested.
- Toggles (default on): `An agent needs me`, `An agent finishes`, `A merge is requested` ([Notifications](./notifications.md)).

## Maintenance

On the **Global** tab: Podman and the shared images serve every workspace, so a rebuild from one reaches new agents of all of them. Every workspace's Foreman shows the same state (built 2026-10-18): one builds at a time (a lock in `~/.local/share/foreman/images/`; a second one waits for it, then finds the image built), and each reads the build's state and log from there, so a build running for another workspace shows as `building…` here, and one whose Foreman stopped mid-way as `failed`, with its log. When the image chain lands, the **Workspace** and project layers, each built from its own Containerfile, show with their workspace (its Workspace tab) and project instead; Base and Harnesses stay here. **Phasing:** phase 1 has one image for every project ([Agent images](../architecture/tech-stack.md#agent-images)), so the section shows Podman's state and that one image: its state, its last build log, and **`Rebuild`** (without cache, picking up the latest Claude Code). Phase 1 shows the state without the build time, and no harness versions yet (neither here nor on the Harnesses rows). The chain rows, Refresh with prune, Customize hints and Prune below are the target, built with the [full image chain](../../backlog/image-chain.md).

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

On the **Global** tab. `Theme`: segmented `System` / `Dark` / `Light`, default `System` (follows the operating system's `prefers-color-scheme`, live). The same in every workspace: one of the CEO's preferences, in `~/.local/share/foreman/preferences.toml` (`GET/PUT /api/preferences`). It shows at once, is put back if the save is refused, and another workspace's open tab picks it up when it comes back into focus. The browser keeps a copy only to paint the first frame ([UI state](../models/ui-state.md), [Design system](../design/design-system.md)).
