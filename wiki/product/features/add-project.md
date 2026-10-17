---
type: spec
status: partial
tags: [feature, ui, workspace, config]
---

# Add a project

A modal ([App shell](./app-shell.md#overlays)) opened from **`+ New project`** in the sidebar, and from the **A project** row of the [setup checklist](./first-run.md#the-checklist). It creates a project in a new workspace folder, or turns an existing one into a project, by creating its [project repo](../models/project-config.md#the-project-repo), `project/`. **Foreman never clones**: the CEO puts code repos in a folder first ([Workspace](../models/workspace.md)), or has Foreman create an empty one (the CEO, 2026-10-16: on a first run there is never a folder to adopt, so a new one comes first).

## Content

- Title "New project".
- Both fields of a new folder are always shown, laid out like the [Deploy dialog](./deploy-team.md)'s fields (the CEO, 2026-10-17: hiding a field that has to be filled anyway is odd):
  - **Project name**: the label, a Mono input, and under it `Only letters, digits, dots, hyphens and underscores, starting with a letter or digit.` The rules are a repo's: letters of either case (the CEO, 2026-10-16: workspace folders such as `DevProject` already have them), digits, `.`, `-` and `_`, starting with a letter or digit, at most 63 characters, never hidden. No folder of that name may exist, in any case: a Mac's filesystem takes `Ledger` and `ledger` for one, and elsewhere they would be two names hard to tell apart. As the name is typed, the dialog checks it against the projects and the candidates (the CEO, 2026-10-17): `Ledger is already a project.` or `DevProject/ already exists: pick it below to add it.` replaces the rules hint, in `--err`, and disables the button. Hidden folders and plain files are only caught by the server.
  - **Repo name**: a Mono input and the hint `Creates <project>/<repo>/ and initializes a git repo in it, with a first commit. Leave it blank to add one later.` (the names as typed, the placeholders until then). It is prefilled with the name and follows it as it's typed, until the CEO edits it. Disabled and empty while a row is selected.
- Below them, with some space, only when there are some, **`or add an existing folder (Foreman never clones: put your repos in it first)`** and one selectable row per **candidate**: a direct subfolder of the workspace without `project/project.toml` (`GET /api/candidates`, [API](../architecture/api.md)). Hidden folders are not candidates. Row: padding `12px 14px`, radius 9; selected = `--acc` border on `--accSoft`.
  - `<folder>/` (Mono 500 14px)
  - The detail: `2 git repos: billing-api, billing-web`, `1 git repo: mobile-app`, or `No git repos yet`.
- Typing a name clears the selected row, and selecting a row clears the name: the dialog does one or the other.
- The selected row grows the note `Creates <folder>/project/: a git repo with project.toml and a starter wiki.` and, when the folder has no git repo, its own **Repo name** field, prefilled with the folder name, with the same hint.
- A blank Repo name adds the project without a repo; [Project settings](./project-settings.md) can add one later, and deploying refuses until there is one.
- Buttons: `Cancel`, and primary **`Create project`** (a new folder) or **`Add project`** (a selected row), disabled until there is a name or a row, or while the name clashes.

## Effect

`POST /api/projects {folder, repo?, new?}`:

1. With `new`, creates the folder `<folder>/` in the workspace (refused if the name breaks the rules or the folder exists). Without it, the folder must be a candidate.
2. Creates `<folder>/project/` and runs `git init` in it.
3. Writes `project/project.toml` with the defaults ([Project config](../models/project-config.md)): the folder name as `project`, no `repos` (so every git repo in the folder is one, including those added later), `main = "main"`, `merge_to_main = "ceo"`, a `[forge]` table with `kind = "none"`, an empty `checks`, and the gates as a commented-out starter list (`# always_ask_ceo = ["new dependency", "touches shared staging"]`) so the CEO sees the option.
4. Creates the **scaffold**, the project repo's files besides `project.toml` (one definition, also used to [fill a project repo that lacks them](./wiki-reader.md#no-wiki-yet)), here the starter wiki `project/wiki/`: the minimum the wiki engine needs to find the bundle and the [board](./board.md) needs to show something: a `wiki.toml` (`spec`, plus one `Backlog` board over `/backlog`) and an `index.md` titled with the project name, saying "Deploy a team from this project's Settings tab." The lead creates everything else. Then commits `project.toml` and the wiki as the repo's first commit.
5. With `repo`: creates `<folder>/<repo>/`, an empty code repo on `main` with a `README.md` in its first commit (so agents' branches have a `main` to start from). The name follows the same rules, is never `project` (in any case), and no folder of that name may exist in any case. (`.sandboxes/` is added to the git excludes at deploy, when it is made: [Sandboxes and git policy](../architecture/sandboxes-and-git.md).)
6. Marks every existing wiki entry as seen (not built yet: it needs the [UI state](../models/ui-state.md) API) ([UI state](../models/ui-state.md)), navigates to the new project's [Wiki reader](./wiki-reader.md) and shows the "added" toast ([App shell](./app-shell.md#toasts): "Deploy a team from its Settings tab.", not the prototype's "from Sessions").

Nothing is written if any step fails: a new folder is removed whole, and in an existing one `project/` and the new repo are. The error is shown in the dialog.
