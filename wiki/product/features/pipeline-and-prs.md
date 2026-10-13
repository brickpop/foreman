---
type: spec
status: planned
tags: [feature, pipeline, git, forge]
---

# Pipeline and pull requests

How work moves from a member's worktree to `main`. Only the team **lead** proposes changes to `main`, as a pull request on the project's forge; only the **CEO** merges, on the forge or with git: never Foreman. Branch rules: [Sandboxes and git policy](../architecture/sandboxes-and-git.md). Forge calls: [Forge adapters](../architecture/forge-adapters.md).

The pipeline has two kinds of entries, both shown in [Changes](./changes.md):

- **Work items** ([model](../models/work-item.md)): one agent's branch on its way into the lead branch. Informational only: a handoff never creates a request and needs nothing from the CEO.
- **Pull requests** ([model](../models/pull-request.md)): the lead's proposal to merge the lead branch into `main`, carrying every work item integrated since the last one. A PR is the only pipeline entry that raises a [request](./requests.md): the **review**.

Approvals, questions, findings and blocked requests are independent of the pipeline: agents and Foreman raise them whenever they happen, typically long before any PR exists ([Requests](./requests.md)).

## Work item stages

| Stage | Label | Entered when |
|---|---|---|
| 0 | in worktree | A member works on `wt/<agent>/<card>`, the branch for one board card |
| 1 | handed to lead | The member calls `handoff(lead, branch, summary)` ([Team tools](../architecture/team-tools.md)) |
| 2 | lead reviewing | The handoff is delivered into the lead's session. Requesting changes (an `ask` to the author) sends the work item back to stage 0 with a note |
| 3 | integrated | Merged into one of the lead's `lead/<team>/*` branches in the lead's sandbox; checks run |

Each work item carries a short **note**. Changes colors it by a **tone** (`ok`, `err`, `ask`, neutral) derived from the work item's state, never stored, e.g. "checks pass" (`ok`) or "2 review comments" (`ask`).

## Review and merge

Only the lead calls `open_pr(branch, title, summary)`, naming one of its own lead branches (`lead/<own team>/*`; anything else is an error). Foreman runs the project `checks`, creates the **pull request**, and raises its **review** request. The PR carries the work items whose branch head is contained in that lead branch (git ancestry) and that aren't in another PR yet; they leave "In flight" and are listed under it. **Several PRs can be open at once**: a lead can have several lead branches, each with at most one open PR, and other teams' leads on the same project have their own. Calling `open_pr` again on a branch whose PR is open updates that PR (see *When main moves*). **Foreman never merges and has no diff view**: the CEO reviews and merges on the forge, or with git when there is no forge. Foreman never touches the CEO's working copy either; the CEO pulls `main` themselves.

**With a forge** (GitHub, Gitea, Forgejo): Foreman pushes the lead branch, opens the PR on the forge immediately, and records its URL and number. The review request shows the PR line and **`Open PR ↗`** (primary), **`Send back`** and **`Open session`**. Foreman then polls the PR's state ([Forge adapters](../architecture/forge-adapters.md#pr-state)):

| On the forge | Foreman |
|---|---|
| Merged | Marks the PR merged (`merged_at`), resolves the review request, tells the lead. The PR and its work items move to "Merged". Sandboxes fetch the new `main` when next idle |
| Closed without merge | Marks the PR closed, resolves the review request as closed, returns its work items to stage 0, tells the lead |
| Review comments or "changes requested" | Relays them into the lead's session (`⇄` lines, [Sessions](./sessions.md)); the PR stays open |

**Without a forge** (`forge: none`): there is no PR on a server. Foreman publishes the lead branch into the project repo as a plain branch (never checked out) and raises the review request: "<Lead> wants <branch> merged into main in <repo>." There is no merge button: the CEO reviews and merges with git. Foreman watches the project repo and, once its `main` contains the branch head, marks the PR merged, resolves the request and tells the lead.

**`Send back`** exists either way: the CEO writes notes; Foreman posts them as a PR comment (when there is a PR on a forge) and into the lead's session, closes the PR, and returns its work items to stage 0 with the notes. The lead decides who reworks, and opens a new PR when ready.

Chief of Staff config changes (branch `foreman/config-N`, [Chief of Staff](./chief-of-staff.md)) are pull requests too, with no work items: a PR on the forge, or a branch to check without one. Their review request adds **`Discard`**, which closes the PR and deletes the branch.

The forge is chosen per project ([Project config](../models/project-config.md)); its token lives in workspace [secrets](../models/secrets.md). Supported: GitHub, Forgejo, Gitea, `none`.

### When main moves

Open PRs on the same repo race each other: when one is merged, the others may now be behind `main`, or conflict with it. **Foreman never rebases or resolves conflicts itself**; it tells the lead who has to:

- When a PR is merged (seen on the forge, or detected in the project repo without a forge), Foreman writes a line into the session of the lead of **every other open PR targeting `main` of the same repo**: `⇄ main moved (PR #128 merged). Rebase lead/atlas/settlement-v2 onto main and push.` (without a forge, the merged branch's name replaces `PR #128`).
- If the forge reports an open PR as conflicting with `main`, its review request panel shows **conflicts with main** (`--err`) and the lead gets the same line.
- The lead rebases in its sandbox and calls `open_pr` again on the same branch. Foreman fetches it (rewriting a `lead/<team>/*` branch is allowed for its own lead; [Sandboxes and git policy](../architecture/sandboxes-and-git.md#branch-policy)), updates the PR's branch on the forge (or in the project repo), reruns the checks and refreshes the request. The PR, its number and its review request stay the same.
