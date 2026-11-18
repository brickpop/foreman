---
type: task
status: in-progress
priority: medium
tags: [feature, ui]
epic: workflow
---

# Project files, and opening folders on the CEO's desktop

**Why:** the CEO wants to hand files to a project's agents (a PDF, a dataset, screenshots) and give the agents a place for data that outlives a task and a team; and to open an agent's folder in the desktop's file manager instead of copying its path (the CEO, 2026-11-06). Each agent already has a private folder that persists ([its sandbox](../product/architecture/sandboxes-and-git.md#layout)), but nothing was shared by the project's agents whatever team is deployed.

## Decided (the CEO, 2026-11-06)

- **One `files/` per project**, beside its HQ: `~/foreman/projects/<id>/files/` (was `<workspace>/files/` until [One Foreman folder](./foreman-folder.md)), kept across teams and deployments. Two folders in it (`inbox` is fine scoped there; the project's Inbox section and the wiki's `inbox/` keep their meaning):
  - **`files/inbox/`**: the CEO's files for the agents. Read-only for every agent.
  - **`files/shared/`**: the agents' persistent data, read and written by every agent of the project. Not git: no history.
- **The session prompt** tells each agent where both are and what each is for (data later work needs goes in `shared/`; knowledge still goes in the wiki).
- **Open on the desktop**: Foreman opens a folder with the host's `open` (macOS) or `xdg-open` (Linux, with a display). Where it can't, no Open button shows at all.
- **Session header**, at its right end, three labelled buttons alike, a click opening that folder: **`Agent folder ↗`** (the shown agent's), **`Shared files ↗`**, **`File inbox ↗`** last. They replaced the agent's folder copy button and its Open icon: nothing copies the path any more (the CEO, 2026-11-06). Each folder is made if missing.
- Every project has its `files/` (since [One Foreman folder](./foreman-folder.md), 2026-11-15: the legacy projects without one are gone).

## Safety

- The browser never sends a path: the server derives each folder (an instance's sandbox; the project's `files/<inbox|shared>`), and refuses anything but a real directory (a symlink, a file, a missing agent folder). The program runs directly, never through a shell, the folder its only argument.
- The endpoints are POSTs behind the guard (token, Origin, Host): a website can't trigger them.
- Agent ids are slugs (no `.app` bundle can be opened by macOS's `open` as an app); the symlink refusal covers an agent replacing `inbox`/`shared` on the Direct engine.
- `files/shared/` lets any agent change what another left there, bypassing the lead, and a misled agent could plant instructions there: accepted (the CEO, 2026-11-06), recorded in [Security](../product/architecture/security.md#accepted-limitations-v1).

## Steps

- [x] spec: [Sandboxes and git policy](../product/architecture/sandboxes-and-git.md) (layout, mounts), [Project config → The project's folder](../product/models/project-config.md#the-projects-folder), [Session prompt](../product/architecture/session-prompt.md), [Sessions](../product/features/sessions.md) (header), [API](../product/architecture/api.md), [Security](../product/architecture/security.md)
- [x] server: `desktop` package (detects `open` / `xdg-open` + display; opens a directory); tests
- [x] server: `files/inbox` and `files/shared` made at each agent start and mounted (inbox ro, shared rw); tests
- [x] server: `GET /api/desktop` ({canOpen}), `POST /api/desktop/open` ({instance} or {project, folder}); tests
- [x] prompt: the two folders in This assignment; tests
- [x] ui: the three buttons in the Session header, shown only with `canOpen`; client test
- [x] contributions (the CEO, 2026-11-18): `contribute()` saves a draft in `files/shared/contributions/`, uncommitted, and still tells the lead, who files it in the wiki or drops it; the HQ's `inbox/` is gone from the starter (`AGENTS.md`, `wiki.toml`), the prompt and the specs ([Team tools](../product/architecture/team-tools.md#tools)), so `inbox/` means only the CEO's files. Existing projects keep their HQ `inbox/` and the old `AGENTS.md` wording until migrated by hand; tests
- [ ] the CEO tries it on macOS and on Fedora: the three buttons open Finder / the file manager; inside an agent, `touch ~/foreman/projects/<id>/files/inbox/x` fails and `files/shared/x` works; the prompt (Session → prompt) names both. Then fold this card into the specs and delete it
