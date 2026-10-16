---
type: spec
status: partial
tags: [feature, ui, workspace, config]
---

# Add a project

A modal ([App shell](./app-shell.md#overlays)) opened from **`+ New project`** in the sidebar. It turns an existing workspace subfolder into a Foreman project by creating its [project repo](../models/project-config.md#the-project-repo), `project/`. **Foreman never clones**: the CEO puts the code repos in the folder first ([Workspace](../models/workspace.md)), or has Foreman create an empty one for a folder that has none.

## Content

- Title "Add a project". Subline: `Folders in ~/agents-ws that aren’t projects yet. Clone your repos into a folder, then add it here.` (with the real workspace path).
- One selectable row per **candidate**: a direct subfolder of the workspace without `project/project.toml` (`GET /api/candidates`, [API](../architecture/api.md)). Hidden folders are not candidates. Row: padding `12px 14px`, radius 9; selected = `--acc` border on `--accSoft`.
  - `<folder>/` (Mono 500 14px)
  - The detail: `2 git repos: billing-api, billing-web`, `1 git repo: mobile-app`, or `No git repos yet`.
- **Empty**: `No new folders found in ~/agents-ws.`
- When a row is selected, a note: `Creates <folder>/project/: a git repo with project.toml and a starter wiki.`
- When the selected folder has no git repo: a **New repo** field (Mono), prefilled with the folder name, labelled `New repo (agents need one to work on; Foreman creates it empty, with a first commit. Leave it blank to add one later.)`. Clearing it adds the project without a repo; [Project settings](./project-settings.md) can add one later, and deploying refuses until there is one.
- Buttons: `Cancel`, and primary **`Add project`** (disabled until a row is selected).

## Effect

`POST /api/projects {folder, repo?}`:

1. Creates `<folder>/project/` and runs `git init` in it.
2. Writes `project/project.toml` with the defaults ([Project config](../models/project-config.md)): the folder name as `project`, no `repos` (so every git repo in the folder is one, including those added later), `main = "main"`, `merge_to_main = "ceo"`, a `[forge]` table with `kind = "none"`, an empty `checks`, and the gates as a commented-out starter list (`# always_ask_ceo = ["new dependency", "touches shared staging"]`) so the CEO sees the option.
3. Creates the starter wiki `project/wiki/`: the minimum the wiki engine needs to find the bundle and the [board](./board.md) needs to show something: a `wiki.toml` (`spec`, plus one `Backlog` board over `/backlog`) and an `index.md` titled with the project name, saying "Deploy a team from this project's Settings tab." The lead creates everything else. Then commits `project.toml` and the wiki as the repo's first commit.
4. With `repo`: creates `<folder>/<repo>/`, an empty code repo on `main` with a `README.md` in its first commit (so agents' branches have a `main` to start from). The name is lowercase letters, digits, `.`, `-` and `_`, never `project`, and must not exist yet. (`.sandboxes/` is added to the git excludes at deploy, when it is made: [Sandboxes and git policy](../architecture/sandboxes-and-git.md).)
5. Marks every existing wiki entry as seen (not built yet: it needs the [UI state](../models/ui-state.md) API) ([UI state](../models/ui-state.md)), navigates to the new project's [Wiki reader](./wiki-reader.md) and shows the "added" toast ([App shell](./app-shell.md#toasts): "Deploy a team from its Settings tab.", not the prototype's "from Sessions").

Nothing is written if any step fails; the error is shown in the dialog.
