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

1. Writes `<folder>/.foreman/project.yaml` with the defaults ([Project config](../models/project-config.md)): the folder name as `project`, the detected repos, `wiki: ./wiki`, `main: main`, `merge_to_main: ceo`, `forge: {kind: none}`, an empty `checks`, and the gates as a commented-out starter list (`# always_ask_ceo:` with `# - new dependency`, `# - touches shared staging`) so the CEO sees the option.
2. If there is no `wiki/`, creates the minimum the wiki engine needs to find the bundle and the [board](./board.md) needs to show something: a `wiki.toml` (`spec`, plus one `Backlog` board over `/backlog`) and an `index.md` titled with the project name, saying "Deploy a team from this project's Settings tab." The scrum master creates everything else.
3. Adds `.sandboxes/` to the git excludes ([Sandboxes and git policy](../architecture/sandboxes-and-git.md)).
4. Marks every existing wiki entry as seen ([UI state](../models/ui-state.md)), navigates to the new project's [Wiki reader](./wiki-reader.md) and shows the "added" toast ([App shell](./app-shell.md#toasts): "Deploy a team from its Settings tab.", not the prototype's "from Sessions").

Nothing is written if any step fails; the error is shown in the dialog.
