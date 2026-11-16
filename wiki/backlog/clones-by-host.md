---
type: task
status: todo
priority: low
tags: [feature, decision]
epic: install-first-run
---

# Clones: two remotes with the same path

**Why:** the CEO, 2026-11-16, on [Portable projects](./portable-projects.md)' part 3. A code repo Foreman clones goes to `repos/<the URL's path>` ([Add a project → Cloning a repo](../product/features/add-project.md#cloning-a-repo)), so `github.com/me/app` and `gitlab.com/me/app` map to the same folder: the second is refused ("holds another repo"), and the CEO picks a folder for it elsewhere. Every project using the same repo already shares one clone, which the CEO wants kept ("maybe it's ok to have all in one place").

## To decide

- **The host in the path** (`repos/github.com/me/app`): no collision ever, one more level everywhere. Clones made before stay where they are (Foreman finds them by path, recorded per machine), so only new clones change.
- **A suffix on collision** (`repos/me/app-2`): paths stay short; the second one's place depends on which came first, so two machines may disagree.
- **Leave it**: picking a folder covers the rare case.

## Steps

- [ ] the CEO picks one
- [ ] `cloneDir` and its tests; the spec's Cloning a repo
