---
type: task
status: todo
priority: high
tags: [feature]
epic: install-first-run
---

# Restructure the New project dialog

**Why:** the CEO, 2026-10-26: "very confusing", "very badly structured". Creating a project is one of the first things a new CEO does ([First run](../product/features/first-run.md#the-checklist)), so it has to be clear. What the CEO hit, checked against the code (`ui/src/components/NewProject.tsx`, `AddRepo.tsx`, `RepoPicker.tsx`):

- **Repos already added are hard to see.** Step 2 lists them only as one line of text above the form (`Added: api, web. Add another, or finish.`), and the form, once it adds a repo, goes back to the folder browser. There is no list of the project's repos with their paths and main branches.
- **The browser offers repos already added.** `RepoPicker` marks a git repo's folder `git` and lets the CEO **Pick** it, even when it is already in the project. Only the server refuses it afterwards ("not already in the project", [Add a project → Code repos](../product/features/add-project.md#code-repos)). The same goes for Project settings, which uses the same `AddRepo`.
- **Nothing asks for a profile.** A new project gets the default profile without saying so ([Add a project → Effect](../product/features/add-project.md#effect), step 5). The only place to choose one is [Project settings](../product/features/project-settings.md) (`ProfileChoice`), which the CEO has to know to look for.
- **The overall flow is unclear.** The project is created in step 1, before its repos. The project (its Wiki) opens behind the dialog while step 2 is still open. **Later** and **Done** both just close the dialog.

Was blocked by [Project workspaces](./archive/project-workspaces.md), built 2026-10-28: step 1 already has a typed **Workspace** field (following the name, `GET /api/folders/workspace`); this card was to add its **Change**; [One Foreman folder](./foreman-folder.md) removes the field instead (every project's folder is `~/foreman/projects/<id>/`), and [Portable projects](./portable-projects.md) adds Clone beside picking a repo.

## Decided (the CEO, 2026-11-04), replacing step 2 below

The dialog is one step: once the project is created it closes and the project's [Repos](../product/features/repos.md) page opens, which already is the list of its repos (name, path, default branch, Remove) with **Add a repo**. Built 2026-11-04, with the one-step repo form and the breadcrumb browser ([Add a project → Picking repos](../product/features/add-project.md#picking-repos)). Left in this card: the workspace's folder browser, the profile in step 1, and repos already added greyed in the browser's list (the folder shown already greys **Add repo**, 2026-11-04).

## Decided (the CEO, 2026-10-28)

- **Two steps.** Step 1, **New project**: the name, the **workspace** (pre-filled `<projects folder>/<name>`, editable; offers to create the projects folder when it's missing), the project repo (New | Clone, always into `<workspace>/project/`), and the **profile** (a select, default preselected, with its harness and connected state, as in Project settings). **Create project**.
- Step 2, **`<name>`: code repos**: a real list of the project's repos (name, path, main branch, **Remove**), then **+ Add a repo**, which opens the add form (`AddRepo`, shared with Project settings) under the list. Each repo is checked by the server as it's added. One button: **Done: open `<name>`**.
- **Repos already in the project** show in the folder browser greyed and marked `added`, and opening one says it's added, with **Add repo** off; the server marks them by real path, so a symlink to one counts.
- **The project (its Wiki) opens when the dialog closes** (Done, or closing step 2), not behind it at creation. Cancel in step 1 opens nothing.

```
── New project ─────────────────────────────────────
Project name  [ledger            ]
Workspace     [~/workspace/foreman/ledger] [Change]
Project repo  (New) Clone
Profile       [default · Claude Code ▾] connected
                         [Cancel] [Create project]

── ledger: code repos (step 2 of 2) ────────────────
api   ~/dev/api       main     [Remove]
web   ~/dev/web       trunk    [Remove]
[+ Add a repo]
  (picker opens here; added repos greyed)
                           [Done: open ledger]
```

**Done when:** the dialog is as decided above, and [Add a project](../product/features/add-project.md) (with [First run](../product/features/first-run.md) and [Project settings](../product/features/project-settings.md), which share `AddRepo`) describes it.

- [x] agree on the structure with the CEO
- [x] ~~step 2, `AddRepo`'s list~~: the Repos page instead (2026-11-04)
- [ ] the browser's `added` (the server marks the entries, given the project); the profile in step 1
- [ ] tests: an added repo can't be picked again (through a symlink too); the profile chosen is the one the project gets; the Repos page opens once the project is created
- [ ] update the specs above
