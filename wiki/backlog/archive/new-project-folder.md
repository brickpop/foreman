---
type: task
status: done
priority: high
tags: [feature]
epic: install-first-run
---

# New project: create the folder, not only adopt one

**Why:** the CEO, 2026-10-16. The **New project** dialog only lists existing workspace folders ("Folders in … that aren't projects yet. Clone your repos into a folder, then add it here"). With none, it complains instead of creating a project, which is what the button promises. On a first run there is never such a folder ([Add a project](../../product/features/add-project.md), [First run](../../product/features/first-run.md)).

**Done when:** the dialog leads with **New project**:
- A name. Foreman creates `<workspace>/<name>/`, its project repo (`project/`, as today) and, by default, an empty code repo named after it (the same field as today's New repo).
- Below it, **or add an existing folder**, listing the candidates only when there are some.
- The empty-state complaint goes.
- `POST /api/projects` takes a new folder name as well as an existing one. Foreman still never clones.

- [x] spec (Add a project), API, the dialog; the name's rules match a repo's (lowercase letters, digits, `.`, `-`, `_`; not hidden; must not exist)
- [x] tests: a new folder, its repos and first commits; a name that exists or is invalid is refused with nothing written

**Built 2026-10-16:** `POST /api/projects {folder, repo?, new: true}` (the workspace's `NewProject`); the dialog leads with a name and lists the candidates below it, only when there are some. Building it showed that adding an existing folder whose new code repo failed left that half-made repo behind; both paths now share one step that removes what it made.
