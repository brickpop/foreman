---
type: spec
status: partial
tags: [feature, ui, home, config, security]
---

# Settings screen

The **Settings** screen (sidebar footer), in **tabs by topic** (the CEO, 2026-10-20: with no workspace any more, the old `Workspace | Global` split said nothing, both being machine-wide; that workspace was the folder Foreman once ran on, not a [project's workspace](../models/project-config.md#the-workspace)). The tab is in the URL, `/settings?tab=git|look|system`; **Agents**, the first, has none, and any other value, the old `workspace` and `global` included, opens Agents. The same tab bar as a project's.

| Tab | Sections |
|---|---|
| **Agents** | [Profiles](#profiles), [Instructions for every agent](#instructions-for-every-agent), [Running agents](#running-agents) |
| **Git & forges** | [Forge accounts](#forge-accounts), [Global repo](#global-repo) |
| **Notifications & look** | [Notifications](#notifications), [Appearance](#appearance) |
| **System** | [Home](#home), [Projects](#projects), [Maintenance](#maintenance) |

**Each section says where it is kept**, after its label (` · Global repo`, `--tx4`, not bold): **Global repo** (`~/.foreman/config/settings.toml`, [Global settings](../models/global-settings.md): the same on every machine the CEO clones it to), **This machine** (profiles, preferences, image builds, in the home), or, for Forge accounts, **Global repo + this machine** (hosts in the global repo, tokens on this machine; [secrets](../models/secrets.md)). Home sets nothing, so it says nothing. When `settings.toml` has problems, a red banner lists them above every tab.

Padding `24px 24px 48px`, max 760px. H1 "Settings"; sections labelled 13px `--tx4`, 36px apart; rows (grid `1fr auto`, padding `14px 0`, bottom border) with a title, an explanation and a control. Changes save immediately ([API](../architecture/api.md), `PUT /api/settings`), except the [instructions](#instructions-for-every-agent) and the [projects folder](#projects), which save on **Save** (Discard drops the edit).

## Instructions for every agent

On **Agents**, kept in the global repo (`instructions` in [Global settings](../models/global-settings.md)). The CEO's own standing rules for coding agents, the way they'd write them in their `~/.claude/CLAUDE.md` or Pi's `AGENTS.md`, which Foreman's agents never read (they run with a private config dir; the CEO, 2026-10-26). Intro: "Your standing rules, as you'd write them in your own CLAUDE.md or AGENTS.md. Foreman adds them at the end of every agent's session prompt, from its next start. Where they conflict with Foreman's own rules (team tools, git, the wiki), Foreman's win."

A full-width text field (8 rows, Mono 13px on `--term`, resizable; placeholder `e.g. Write commit messages in English. Prefer small functions.`). Once edited, **`Save`** (primary) and **`Discard`** appear under it: a long text saves as one commit, never mid-sentence (the CEO, 2026-10-28; the other settings save on change). A failed save keeps the text and says why in a toast. A change on disk shows while the field isn't being edited.

Decided (the CEO, 2026-10-28): **one global text**, not per profile (the same rules usually apply whatever the harness; per profile can come later); **written in Foreman**, not copied from the CEO's own files (those carry rules for the CEO's own sessions: Mac paths, tools the container lacks); only the CEO's personal layer (a team's is the soul's job description, a project's its repo's `AGENTS.md`); **team agents only**, not the [Chief of Staff](./chief-of-staff.md), whose prompt is fixed; applied **at an agent's next start**, like a profile change. Where it goes in the prompt: [Session prompt](../architecture/session-prompt.md#the-ceos-instructions).

## Running agents

On **Agents**, kept in the global repo.

| Row | Explanation | Control |
|---|---|---|
| Running sessions at once | Extra sessions wait in a queue until a slot frees up. | Stepper, 1–64, default 12 ([Agent runtime](../architecture/agent-runtime.md#concurrency)) |
| Stuck after | An agent mid-turn that sends no sign of activity (a prompt, a tool call, a turn ending) for this long gets a card on the Dashboard. It keeps running; you decide. | Select `5`, `10`, `15`, `20` (default), `30`, `45`, `60`, `90`, `120 min`, plus the file's own value when written by hand ([Agent runtime → Stuck agents](../architecture/agent-runtime.md#stuck-agents)) |
| Session logs per agent | Every byte each agent's terminal printed, kept in Foreman's home. Past this size, the oldest go first. | Select `10`, `25`, `50` (default), `100`, `250`, `500`, `1000 MB`, plus the file's own value ([Persistence → Session logs](../architecture/persistence.md#session-logs)) |
| Run agents in | Containers isolate each agent from your machine. Without them, agents run as plain processes with full access to it. | Segmented `Containers (Podman)` / `Directly (no isolation)`. Choosing Directly asks for confirmation; while agents run directly, a warning stays visible here. **Takes effect when Foreman restarts** (decided 2026-10-15): until then the row says `Restart Foreman to switch` and Maintenance shows the engine still running. `Containers` can't be chosen while Podman isn't installed (on macOS: no machine), and the row says why: Foreman wouldn't start with it ([Global settings](../models/global-settings.md)) |

## Home

On **System**. A fact, not a setting.

| Row | Explanation | Control |
|---|---|---|
| Home | Foreman’s folder on this machine: the global repo (teams, settings), the agents’ harness folders (their config and conversations), and its state. Your projects live in their workspaces. | The path (`GET /api/home`, [Home](../models/home.md)), read-only (Mono 13px `--tx2`) |

## Projects

On **System**, kept in the global repo.

| Row | Explanation | Control |
|---|---|---|
| Projects folder | New project suggests each project's workspace here, in a folder of its name; you can pick another. ~ is your home folder, on any machine. | A path field (`projects_folder`, [Global settings](../models/global-settings.md)); **Save** and **Discard** appear once it is edited |

## Profiles

On **Agents**, kept on this machine. "Each profile is a harness and its credentials, yours on this machine: Claude Code's login, or the model server Pi runs on. Every project can use it; a project uses default unless its settings pick another." See [Session runner → Credentials](../architecture/session-runner.md#credentials).

Rows (grid `1fr auto auto`): the profile's name (Mono 600) and its harness label (13px `--tx4`); its state, `connected` (`--ok`) or `not connected` (`--tx4`); **`Connect`** / **`Reconnect`**, and **`Delete`** on every profile but `default`. Delete asks first: "Delete the <name> profile? Its credentials go with it. Projects that use it can't start agents until they pick another profile." No rename for now: projects' records name it.

Under the rows, **`New profile`** turns into a form: `Name` (Mono, placeholder `work`; what is typed is made into a profile name, shown beside it when it differs: `My Work` → `my-work`), `Harness` (a select of the supported harnesses: Claude Code, Pi), `Cancel` and `Create`. Creating opens the profile's Connect dialog straight away, since a profile without credentials can't run agents. `GET /api/profiles`, `POST /api/profiles`, `DELETE /api/profiles/:name` ([API](../architecture/api.md)).

**`Connect`** / **`Reconnect`** opens the **Connect dialog** (`ConnectDialog`, 760px, title `Connect <profile>`): for a harness whose credentials are a login (`connect: "login"` in `GET /api/harnesses`), the login container's terminal running the profile's harness login (for Claude Code: Claude Code itself, where the CEO types `/login`, since it never asks on its own). Under the terminal, the login state, followed while open (`Not connected yet.` in `--tx4`, then `<profile> is connected.` in `--ok`), and one button: `Cancel`, which becomes a primary `Done` once connected. It closes only through that button, never Escape or the backdrop: Escape belongs to the terminal, and a stray click mustn't end the login. Closing ends the login container (`DELETE /api/profiles/:name/connect`), logged in or not. The same dialog opens from [Deploy a team](./deploy-team.md) and the setup checklist ([First run](./first-run.md)).

**For Pi** (`connect: "endpoint"`; the CEO, 2026-10-18) the same dialog (560px) is a form: "A server with an OpenAI-compatible API (Ollama, LM Studio, llama.cpp, vLLM…) and the model Pi runs on it. A server on this machine is reached from the agents' containers for you." Fields: `Server URL` (Mono, placeholder `http://localhost:11434/v1`) with **`Detect models`** beside it, which lists the server's models (`POST /api/profiles/:name/endpoint/models`, asked by Foreman itself; the server's own error in `--err` when it can't, "The server offers no model: type its id." when it lists none); `API key` (optional, write-only: placeholder `none`, or `set: leave empty to keep it`); `Model`, a select of the detected models (the first chosen unless the current one is among them) or, before any Detect, a text field; `Context window (tokens)` and `Longest reply (tokens)`, prefilled with 262144 and 131072 for a new profile. A note: "Saved as Pi's `models.json` in the profile's folder. Fields added there by hand (sampling, thinking levels) are kept while the model stays the same." `Cancel` and **`Save`** (`PUT /api/profiles/:name/endpoint`; refused with the reason: a URL that isn't http(s), no model, sizes not above 0, a reply longer than the context). Escape and the backdrop close it, as any form. The profile is connected once saved ([Harness adapters → Pi](../architecture/harness-adapters.md#pi)). **Phasing:** `reconnect needed` (an expired login) needs the adapter to detect it, and each harness's detection status in the agent image (`found · <version>`) comes with the [full image chain](../../backlog/image-chain.md).

## Forge accounts

On **Git & forges**, hosts in the global repo, tokens on this machine. "Tokens Foreman uses when you ask it to create a PR or sync a repo, and to watch those PRs. They stay on this machine. Each project's config names its forge."

One row each for **GitHub**, **Forgejo** and **Gitea**: the host and owner (or `—`), the token status (`token set` `--ok`, or `not connected` `--tx4`), and **`Connect`** / **`Change`**. Connect or Change opens an inline form: host URL (prefilled `github.com` for GitHub) and a write-only token field; Save validates the token against the forge and stores it ([Forge adapters](../architecture/forge-adapters.md), `PUT /api/secrets/forge/:kind`). A stored token is never shown again.

## Global repo

On **Git & forges**, kept in the global repo. "Where your team definitions and settings are hosted, so Sync can push them there. Without a forge they stay on this machine. Nothing is pushed automatically." Where the [global repo](../models/home.md#the-global-repo) is hosted, so its `main` can be pushed there: `No forge` (the default; it stays on this machine) or one of the connected forges, with the repo's URL (`global_forge` in [Global settings](../models/global-settings.md#schema)). With a forge set, a small outline **`Sync`** button fetches the forge's `main`, merges it in and pushes the global repo's `main` there, stopping with a message on a conflict; nothing is pushed automatically ([Forge adapters → Sync](../architecture/forge-adapters.md#sync-the-reserved-repos)). **Phasing:** Sync comes with the [Git epic](../../backlog/epics/git.md); until then the button shows, disabled.

## Notifications

On **Notifications & look**, kept in the global repo.

- **Browser notifications**: the permission state (raised only while the tab is hidden: when the CEO is looking, the toast says it already), `on` (`--ok`), `blocked` (`--err`), `turn on` (`--accTx`) or `n/a` (`--tx4`, unsupported browser), and a **`Request`** button shown only when permission can still be requested.
- Toggles (default on): `An agent needs me`, `An agent finishes`, `A merge is requested` ([Notifications](./notifications.md)).

## Maintenance

On **System**, kept on this machine: Podman and the shared images serve every project, so a rebuild reaches new agents of all of them. One build runs at a time (a lock in `~/.foreman/images/`; a second one waits for it, then finds the image built), and the build's state and log are kept there, so one whose Foreman stopped mid-way shows as `failed`, with its log (built 2026-10-18). When the image chain lands, the **global** layer (the CEO's own tools) and each project's layer, each built from its own Containerfile, show here too, and each project's also with its project. **Phasing:** phase 1 has one image for every project ([Agent images](../architecture/tech-stack.md#agent-images)), so the section shows Podman's state and that one image: its state, its last build log, and **`Rebuild`** (without cache, picking up the latest Claude Code). Phase 1 shows the state without the build time, and no harness versions yet (neither here nor on the Harnesses rows). The chain rows, Refresh with prune, Customize hints and Prune below are the target, built with the [full image chain](../../backlog/image-chain.md).

"The images agent sessions run in. Foreman builds them as a chain: base, harnesses, your own tools, each project. A layer without its own Containerfile simply passes the one above through."

At the top, Podman's state: `Podman 5.x · running` (`--ok`), or on macOS the machine's state (`machine running` / `starting…` / `stopped`), from [Session runner → The Podman service](../architecture/session-runner.md#the-podman-service).

Then one row per image of the chain ([Agent images](../architecture/tech-stack.md#agent-images)), indented to show it: **Base**, **Harnesses**, **Global**, then one row per project. Each row shows:

- the image name (Mono, e.g. `localhost/foreman/harness`) and its source: `built in`, the Containerfile path, or `transparent` (`--tx4`)
- the state: `ready · built <relative time>` (`--ok`), `building…` (`--team`), `failed` (`--err`) or `not built yet` (`--tx4`)
- on the Harnesses row, the versions inside (`claude 2.1.4 · codex 0.48.0 · …`)
- **`Refresh`** (Base and Harnesses): rebuild with the latest base image and harness CLIs, then rebuild everything below, with an option "Also prune outdated images". Running sessions keep their image; new and restarted sessions use the refreshed one.
- A transparent row shows where its Containerfile would go, so the CEO knows where to create one.
- **`Log`**: the last build log, in a read-only Mono panel.

Below the list: **`Prune`**, with the space it would reclaim. It removes Foreman's outdated images and build cache only, never other images ([Agent images](../architecture/tech-stack.md#agent-images)).

A failed build also raises a **blocked** [request](./requests.md), so it reaches Home.

## Appearance

On **Notifications & look**, kept on this machine. `Theme`: segmented `System` / `Dark` / `Light`, default `System` (follows the operating system's `prefers-color-scheme`, live). One of the CEO's preferences, in `~/.foreman/preferences.toml`, outside the global repo (`GET/PUT /api/preferences`). It shows at once, is put back if the save is refused, and another open tab picks it up when it comes back into focus. The browser keeps a copy only to paint the first frame ([UI state](../models/ui-state.md), [Design system](../design/design-system.md)).

`Terminal font`: a font and a size, for every agent terminal, also a preference (`terminal_font`, `terminal_font_size` in the same file; the CEO, 2026-10-19). Fonts: **JetBrains Mono** (the default; the setting is the terminals' only, the rest of the UI keeps Geist Mono, the CEO, 2026-10-21), **Fira Code**, **Source Code Pro**, all shipped with Foreman (it works offline), and **System monospace** (SF Mono or Menlo on the Mac, nothing to download). A shipped font is fetched only once picked. Sizes 10 to 24 px, default 13. Open terminals switch at once, once the font has loaded (the terminal measures its cells on the font it has). A value in the file that can't be kept (an unknown font, a size out of range) reads as its default; the other values stay. Geist Mono and IBM Plex Mono are no terminal fonts (the CEO, 2026-10-20: fewer fonts, a lighter app), so a file naming them reads as JetBrains Mono.
