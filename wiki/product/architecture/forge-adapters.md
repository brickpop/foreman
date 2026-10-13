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
| `none` | local | No PR: the lead branch is published into the project repo for the CEO to review and merge with git. A project.yaml without a `forge` key means `none` |

The repo name on the forge equals the repo's folder name, under the `forge.url` owner/org. A per-repo name override is not in v1; it would be an optional field in `project.yaml`, added only when a real project needs it.

## Operations

Every adapter implements the same small set, so kinds are interchangeable. **Foreman never merges**: the CEO merges on the forge ([Pipeline and PRs](../features/pipeline-and-prs.md#review-and-merge)).

| Operation | Used when |
|---|---|
| push branch | `open_pr`: Foreman pushes the named `lead/<team>/<topic>` branch to the forge (a force update when the lead rebased it and calls `open_pr` again) with the workspace token. Agents never push to the forge and never see the token ([Sandboxes and git policy](./sandboxes-and-git.md)) |
| open PR (title, summary, head, base = `main`) → URL, number | `open_pr` |
| read PR state | polling, below |
| read check status | after opening and while polling; shown as "checks pass" on review request panels ([Changes](../features/changes.md)), next to Foreman's own run of the project `checks` |
| comment | CEO **Send back**: the notes are posted on the PR (and written into the lead's session; the pull request is closed and its work items go back to stage 0) |
| close PR | CEO **Discard** on a Chief of Staff config change |

For `none` there is no PR: `open_pr` publishes the lead branch into the project repo as a plain branch, and "comment" writes only to the lead's session.

### PR state

While a [pull request](../models/pull-request.md) is open, Foreman polls it on the forge every 60 seconds (and when the CEO opens Changes): no webhooks, so nothing has to reach Foreman from outside. It reads:

- **merged**: the pull request and its work items are marked merged, the review request resolves, the lead is told; Foreman's mirror fetches the new `main` from the forge, and sandboxes fetch it on next idle. The leads of the other open PRs on the same repo are told to rebase ([When main moves](../features/pipeline-and-prs.md#when-main-moves)).
- **mergeability**: when the forge reports the PR as conflicting with `main`, `conflicting` is set (shown as "conflicts with main") and the lead is asked to rebase; cleared when the forge reports it mergeable again.
- **closed** without merge: the pull request is closed, the review request resolves as closed, its work items go back to stage 0, the lead is told.
- **new review comments** and "changes requested" reviews since the last poll: relayed into the lead's session.

Without a forge, Foreman instead fetches the project repo's `main` on the same interval and checks whether it contains the lead branch head (`git merge-base --is-ancestor`).

## Rules

- **Merge method** is the forge's business: the CEO merges there with whatever method the repo allows. **Conflicts** are the lead's: Foreman reports them and asks for a rebase, but never rebases or resolves them itself; the CEO can also send the PR back with a note.
- **Tokens:** the token needs to push branches, open, read, comment on and close PRs; the minimal scopes per forge are documented here with each adapter. An expired or revoked token raises a **blocked** request on the PR's lead instance, linking to [Settings](../features/settings.md#forge-accounts). The token is set in Settings as host URL + token, checked against the forge on save, and never shown again.
- **Check status** shown on a PR is Foreman's own run of the project `checks`, plus the forge's CI statuses alongside when a forge is configured ([Sandboxes and git policy](./sandboxes-and-git.md#checks)).
