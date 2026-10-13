---
type: spec
status: planned
tags: [architecture, forge, git, pipeline]
---

# Forge adapters

A **forge** is where pull requests live. Foreman connects to an existing one; it never hosts it. The forge is chosen per project in [`project.yaml`](../models/project-config.md) (`forge.kind`, `forge.url`); tokens live in workspace [secrets](../models/secrets.md), set from [Settings](../features/settings.md). The pipeline tracker uses an adapter for every PR operation ([Pipeline and PRs](../features/pipeline-and-prs.md)).

## Kinds

| `forge.kind` | API | Notes |
|---|---|---|
| `github` | GitHub REST | `forge.url` is the owner base, e.g. `https://github.com/acme` |
| `forgejo` | Gitea-compatible REST | e.g. `https://localhost:3000/ledger` |
| `gitea` | Gitea REST | Same API family as Forgejo, so one implementation can serve both |
| `none` | local | PRs live inside Foreman; merging runs `git merge --no-ff` locally |

The repo name on the forge equals the repo's folder name, under the `forge.url` owner/org.

## Operations

Every adapter implements the same small set, so kinds are interchangeable:

| Operation | Used when |
|---|---|
| push branch | `open_pr`: Foreman pushes `lead/<branch>` to the forge with the workspace token. Agents never push to the forge and never see the token ([Sandboxes and git policy](./sandboxes-and-git.md)) |
| open PR (title, summary, head, base = `main`) → URL, number | `open_pr` |
| read check status | after opening and on refresh; shown as "checks pass" on review cards ([Changes](../features/changes.md)) |
| merge | CEO **Merge**; then Foreman fast-forwards the project repo's `main` and notifies the lead. Sandboxes fetch the new `main` on next idle |
| comment | CEO **Send back**: the notes are posted on the PR, written into the lead's session, and the item is reset to stage 0 |

For `none`, "push" and "open" only record the item; "checks" are the project `checks` run by Foreman; "merge" is a local `git merge --no-ff` into `main`; "comment" writes only to the lead's session.

## Open questions

1. **Merge method.** Merge commit (`--no-ff`, as for `none`), squash or rebase on forges? Recommendation: merge commit everywhere, for one behaviour.
2. **Repo name mapping.** "Repo name = folder name" breaks when the forge repo is named differently. Recommendation: allow an optional per-repo override in `project.yaml` only when needed (not in v1).
3. **Token scopes and expiry.** Which minimal scopes per forge, and how an expired or revoked token surfaces (a blocked request on the lead? a Settings warning?). Recommendation: a blocked request on the PR's lead instance with a link to Settings.
4. **Merge conflicts at merge time.** The PR may no longer merge cleanly when the CEO presses Merge (main moved). Recommendation: the merge fails visibly, the item goes back to the lead as a blocked request ("merge conflict on lead/*", already a blocked trigger).

Where check status comes from is owned by [Sandboxes and git policy](./sandboxes-and-git.md#open-questions).
