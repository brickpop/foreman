---
type: spec
status: planned
tags: [architecture, forge, git, pipeline]
---

# Forge adapters

A **forge** is where pull requests live. Foreman connects to an existing one; it never hosts it, and **only goes there when the CEO asks**: Create PR on a [merge request](../models/merge-request.md), or Sync on a reserved repo. The forge is chosen per project in [`project.toml`](../models/project-config.md) (`forge.kind`, `forge.url`, `forge.project_repo`), and for the global repo in [Global settings](../models/global-settings.md#schema) (`global_forge`); tokens live in Foreman's [secrets](../models/secrets.md), set from [Settings](../features/settings.md). The pipeline uses an adapter for every forge operation ([Pipeline and merges](../features/pipeline-and-merges.md)).

## Kinds

| `forge.kind` | API | Notes |
|---|---|---|
| `github` | GitHub REST | `forge.url` is the owner base, e.g. `https://github.com/acme` |
| `forgejo` | Gitea-compatible REST | e.g. `https://localhost:3000/ledger` |
| `gitea` | Gitea REST | Same API family as Forgejo, so one implementation can serve both |
| `none` | local | No forge: a review merges locally, with no Create PR, and there is nothing to Sync. A `project.toml` without a `forge` table means `none` |

The repo name on the forge equals the repo's name in the project, under the `forge.url` owner/org. The [project repo](../models/project-config.md#the-project-repo) is named in full by `forge.project_repo`, and the [global repo](../models/home.md#the-global-repo) by `global_forge.url`. A per-repo name override is not in v1; it would be an optional field in `project.toml`, added only when a real project needs it.

## Operations

Every adapter implements the same small set, so kinds are interchangeable. Foreman never merges on a forge: after Create PR the CEO merges there ([Pipeline and merges](../features/pipeline-and-merges.md#review-and-merge)).

| Operation | Used when |
|---|---|
| push branch | **Create PR**: Foreman pushes the merge request's branch to the forge (again, as a plain fast-forward, when the lead calls `request_merge` again after merging `main` in) with the forge account's token. **Sync** on a reserved repo pushes its `main` the same way. Agents never push to the forge and never see the token ([Sandboxes and git policy](./sandboxes-and-git.md)) |
| open PR (title, body, head, base = `main`) → URL, number | **Create PR**: the title and body come from the merge request's wiki entry |
| read PR state | polling, below |
| read check status | after opening and while polling; shown as "checks pass" on review request panels ([Needs you](../features/project-pipeline.md#1-needs-you)), next to Foreman's own run of the project `checks` |
| comment, close PR | CEO **Dismiss** on a review whose merge request has a PR: the reason is posted on the PR and the PR is closed (the reason is also delivered to the lead) |

For `none` there is no PR and nothing to push: the CEO uses Merge to main, and Dismiss only reaches the lead's session.

### PR state

While a [merge request](../models/merge-request.md) with a PR is open, Foreman polls the PR on the forge every 60 seconds (and when the CEO opens the project's Inbox): no webhooks, so nothing has to reach Foreman from outside. It reads:

- **merged**: the merge request and its work items are marked merged, the review request resolves, the lead is told; Foreman fetches the new `main` from the forge into the repo (`Git.Fetch`: remote-tracking refs only, never the CEO's checkout), where every worktree sees it. The leads of the other open merge requests on the same repo are told to merge `main` in ([When main moves](../features/pipeline-and-merges.md#when-main-moves)).
- **mergeability**: when the forge reports the PR as conflicting with `main`, `conflicting` is set (shown as "conflicts with main") and the lead is asked to merge `main` in; cleared when the forge reports it mergeable again.
- **closed** without merge: the merge request is rejected, the review request resolves, its work items go back to stage 0, the lead is told.
- **new review comments** and "changes requested" reviews since the last poll: delivered to the lead (typed into its session when idle).

In every repo, with or without a forge, Foreman also reads `main` on the same interval and marks an open merge request merged when merging its branch into `main` would change nothing ([Merge to main](../features/pipeline-and-merges.md#merge-to-main)): the CEO may merge by hand with git, and the forge with any merge method. A merged branch then moves to the new `main`, so pushing it again for a next PR can't be a plain fast-forward: how (a forced push, or a new branch on the forge) is open until [The forge](../../backlog/git-forge.md) is built.

### Sync (the reserved repos)

The global repo and the project repo never push on their own: Foreman commits to them locally (a UI save, a contribution, a card move, the lead's `commit()`), and nothing reaches their forge until the CEO clicks **Sync**. Sync fetches the forge's `main`, merges it into the local `main` (it may hold commits made on the forge), then pushes `main`. On a conflict it stops, changes nothing, and tells the CEO. Sync is offered only when the repo has a forge: in [Settings → Forge accounts](../features/settings.md#forge-accounts) for the global repo, and in [Project settings](../features/project-settings.md) for the project repo.

## Rules

- **Merge method** is the forge's business: the CEO merges there with whatever method the repo allows. **Conflicts** are the lead's: Foreman reports them and asks the lead to merge `main` in (`merge("main")`) and resolve them; it never resolves them itself; the CEO can also reject the merge request with a reason.
- **Tokens:** the token needs to push branches, open, read, comment on and close PRs; the minimal scopes per forge are documented here with each adapter. An expired or revoked token makes the CEO's action fail with a message naming it (on a PR being watched: a **blocked** request on the lead instance), linking to [Settings](../features/settings.md#forge-accounts). The token is set in Settings as host URL + token, checked against the forge on save, and never shown again.
- **Check status** shown on a merge request is Foreman's own run of the project `checks`, plus the forge's CI statuses alongside when a forge is configured ([Sandboxes and git policy](./sandboxes-and-git.md#checks)).
