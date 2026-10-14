---
type: spec
status: planned
tags: [feature, ui, workspace, config]
---

# Add a project

A modal ([App shell](./app-shell.md#overlays)) opened from **`+ New project`** in the sidebar. It turns an existing workspace subfolder into a Foreman project by creating its [project repo](../models/project-config.md#the-project-repo), `project/`. **Foreman never clones**: the CEO puts the code repos in the folder first ([Workspace](../models/workspace.md)).

## Content

- Title "Add a project". Subline: `Folders in ~/agents-ws that aren’t projects yet. Clone your repos into a folder, then add it here.` (with the real workspace path).
- One selectable row per **candidate**: a direct subfolder of the workspace without `project/project.toml` (`GET /api/candidates`, [API](../architecture/api.md)). Hidden folders are not candidates. Row: padding `12px 14px`, radius 9; selected = `--acc` border on `--accSoft`.
  - `<folder>/` (Mono 500 14px)
  - The detail: `2 git repos: billing-api, billing-web`, `1 git repo: mobile-app`, or `No git repos yet`.
- **Empty**: `No new folders found in ~/agents-ws.`
- When a row is selected, a note: `Creates <folder>/project/: a git repo with project.toml and a starter wiki.`, plus ` Agents need at least one repo to work on.` when it has no repos.
- Buttons: `Cancel`, and primary **`Add project`** (disabled until a row is selected).

## Effect

`POST /api/projects {folder}`:

1. Creates `<folder>/project/` and runs `git init` in it.
2. Writes `project/project.toml` with the defaults ([Project config](../models/project-config.md)): the folder name as `project`, the detected repos, `main = "main"`, `merge_to_main = "ceo"`, a `[forge]` table with `kind = "none"`, an empty `checks`, and the gates as a commented-out starter list (`# always_ask_ceo = ["new dependency", "touches shared staging"]`) so the CEO sees the option.
3. Creates the starter wiki `project/wiki/`: the minimum the wiki engine needs to find the bundle and the [board](./board.md) needs to show something: a `wiki.toml` (`spec`, plus one `Backlog` board over `/backlog`) and an `index.md` titled with the project name, saying "Deploy a team from this project's Settings tab." The lead creates everything else. Then commits `project.toml` and the wiki as the repo's first commit.
4. Adds `.sandboxes/` to the git excludes ([Sandboxes and git policy](../architecture/sandboxes-and-git.md)).
5. Marks every existing wiki entry as seen ([UI state](../models/ui-state.md)), navigates to the new project's [Wiki reader](./wiki-reader.md) and shows the "added" toast ([App shell](./app-shell.md#toasts): "Deploy a team from its Settings tab.", not the prototype's "from Sessions").

Nothing is written if any step fails; the error is shown in the dialog.
