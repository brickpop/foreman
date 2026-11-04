---
type: task
status: done
priority: high
tags: [feature]
epic: install-first-run
---

# The repo picker in one step; the agent's branch opens its repo

**Why:** the CEO, 2026-11-04: the dialog to add a repo was clunky. Pick, then Add (greyed until then); a `..` row; a `git [Pick]` row of its own; Mono everywhere; no close button. And from a Session, an agent's branch was plain text: no way to open it, nor to copy it.

**Built (2026-11-04):**

- [x] the folder browser (`FolderBrowser`, replacing `RepoPicker`): breadcrumbs from the home folder, nothing above it; every folder a row that opens it, a branch icon for a repo; dot folders left out (they were already: `..` was the one shown); a pasted path opens its folder, a refused one leaves it where it was ([Add a project → Picking repos](../../product/features/add-project.md#picking-repos))
- [x] `GET /api/folders` says whether the folder shown is a repo (`repo`) and gives `home` (for the crumbs); `parent` gone; with `?project=`, which of its repos the folder already is (`added`, by real path): **Add repo** greys out for it, and Locate refuses another repo's folder
- [x] `AddRepo` in one step: a repo's folder shown → **Add repo**, its name filled in; any other folder → **Create repo** inside it; no Existing / New toggle; normal font
- [x] Locate uses the same browser, with **Locate here**
- [x] (round 2) 16px icons; the list on `--term`; the pasted path out; Foreman's own folders (projects folder, workspaces) shown with their own icon, not opened (`foreman` in `GET /api/folders`, `TestBrowseMarksForemansFolders`)
- [x] every modal has a close `×` ([App shell → Overlays](../../product/features/app-shell.md#overlays))
- [x] New project closes on creation and opens the project's Repos page, replacing the dialog's step 2 ([Restructure the New project dialog](../new-project-dialog.md) keeps the rest)
- [x] the Session header: `<project> · <team> · [branch ▾]`, the team linking to the project's Team section, the branch a menu (hover) of the repos that have it, work first, each opening that repo on it, and Copy (`BranchMenu`, `branchPlaces`, `placeLine`; [Sessions → Agent tabs](../../product/features/sessions.md#agent-tabs))
- [x] tests: `TestBrowse` (a repo's folder, through a symlink too; `home`), `TestBrowseSaysWhatTheProjectHasAdded`, `folderCrumbs`, `branchesOf`

Not seen in a browser by the agent that built it (none in its container): the CEO checks the look.
