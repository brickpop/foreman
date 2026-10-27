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
- **The overall flow is unclear.** The project is created in step 1, before its repos. The Overview opens behind the dialog while step 2 is still open. **Later** and **Done** both just close the dialog.

**Done when:** the dialog has a structure the CEO has agreed to, it shows repos already added and won't let them be picked again, it asks for the profile, and [Add a project](../product/features/add-project.md) (with [First run](../product/features/first-run.md) and [Project settings](../product/features/project-settings.md) if `AddRepo` changes there too) describes it.

**To decide with the CEO before building:**
- **The structure.** Keep two steps (create, then add repos) but with a proper list of repos, or make it one form (name, project repo, repos, profile) that creates everything when the CEO clicks Create. With one form, a failed step means undoing the earlier ones. With two steps, the project exists before it has repos, as now.
- **Where the profile goes.** In step 1 next to the project repo, or with the repos. It defaults to the default profile, and the dialog says which one that is.
- **How repos already added show in the browser.** Greyed out and marked `added`, or hidden. The browser would need the project's repo paths (it compares real paths, so a symlink to an added repo counts as added).
- **What the Overview does meanwhile.** Whether it should open only once the dialog closes.

- [ ] agree on the structure with the CEO, ideally from a sketch
- [ ] the dialog, `AddRepo` and `RepoPicker` changes (and Project settings, which shares `AddRepo`)
- [ ] tests: an added repo can't be picked again (through a symlink too); the profile chosen is the one the project gets; the repos already added are listed when the dialog opens on a project that has some
- [ ] update the specs above
