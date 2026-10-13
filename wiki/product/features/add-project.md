---
type: spec
status: planned
tags: [feature, ui, workspace, config]
---

# Add a project

A modal ([App shell](./app-shell.md#overlays)) opened from **`+ New project`** in the sidebar. It turns an existing workspace subfolder into a Foreman project. **Foreman never clones**: the CEO puts the repos in the folder first ([Workspace](../models/workspace.md)).

## Content

- Title "Add a project". Subline: `Folders in ~/agents-ws that aren’t projects yet. Clone your repos into a folder, then add it here.` (with the real workspace path).
- One selectable row per **candidate**: a direct subfolder of the workspace without `.foreman/project.yaml` (`GET /api/projects/:id/candidates`, [API](../architecture/api.md)). Hidden folders are not candidates. Row: padding `12px 14px`, radius 9; selected = `--acc` border on `--accSoft`.
  - `<folder>/` (Mono 500 14px)
  - The detail: `2 git repos: billing-api, billing-web · no wiki yet`, `1 git repo: mobile-app · has a wiki`, or `No git repos yet · no wiki yet`.
- **Empty**: `No new folders found in ~/agents-ws.`
- When a row is selected, a note: `Adds <folder>/.foreman/project.yaml` then `.` if it has a wiki, or ` and an empty wiki/ with an index.md.` if not; plus ` Agents need at least one repo to work on.` when it has no repos.
- Buttons: `Cancel`, and primary **`Add project`** (disabled until a row is selected).

## Effect

`POST /api/projects {folder}`:

1. Writes `<folder>/.foreman/project.yaml` with the defaults ([Project config](../models/project-config.md)): the folder name as `project`, the detected repos, `wiki: ./wiki`, `main: main`, `merge_to_main: ceo`, `forge: {kind: none}`.
2. If there is no `wiki/`, creates it with a `wiki.toml` and an `index.md`.
3. Adds `.sandboxes/` to the git excludes ([Sandboxes and git policy](../architecture/sandboxes-and-git.md)).
4. Navigates to the new project's [Wiki reader](./wiki-reader.md) and shows the "added" toast.

Nothing is written if any step fails; the error is shown in the dialog.

## Open questions

1. **Defaults written into the new `project.yaml`.** Which values for `checks` (empty?) and `always_ask_ceo` (empty, or a starter list such as "new dependency")? Recommendation: `checks` empty and a commented-out starter list of gates so the CEO sees the option. The `forge` default is owned by [Project config](../models/project-config.md#open-questions).
2. **Starter wiki content.** Should a new `wiki/` get only `index.md` + `wiki.toml` with a default `Backlog` board, or a fuller skeleton (`backlog/`, `adr/`, `standups/`)? A `wiki.toml` is required in any case: without it `bundle.Discover` doesn't find the bundle ([Wiki engine](../architecture/wiki-engine.md)). Recommendation: `index.md` and a minimal `wiki.toml` (spec + one `Backlog` board over `/backlog`); the scrum master creates the rest.
3. **Follow-up copy.** The prototype's toast and starter wiki say "Deploy a team from Sessions", but deploying happens from Project → Settings or the Team page. Recommendation: the corrected copy used in [App shell](./app-shell.md#toasts) ("Deploy a team from its Settings tab.").
