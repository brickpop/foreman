---
type: spec
status: planned
tags: [feature, ui, requests, pipeline]
---

# Changes

The project's actionable tab: what needs the CEO on this project, the work moving towards `main`, and what recently merged. Body padding `28px 40px 64px`, max width 860px, 36px between sections. The project header and tabs are described in [Wiki reader](./wiki-reader.md#project-header-and-tabs).

## 1. Needs you

This project's open [requests](../models/request.md), one **request panel** each. Semantics of each kind: [Requests](./requests.md).

- **Request panel**: `--panel`, radius 12, padding `16px 18px`, gap 12.
- **Header**: the **kind chip first**, Avatar 24 (no ring), the agent name, the context (`<Team> on <project>`), a spacer, and the **source** (Mono 11.5px `--tx4`): `escalate(approval)`, `escalate(question)`, `escalate(finding)`, `open_pr()`, `raised by Foreman` (blocked) or `Chief of Staff`, derived from the request's `kind` and `source` ([Request](../models/request.md)).
- **Text**: the summary, 15.5px.
- **Link line** (optional, Mono 12.5px `--accTx`), a real link where there is a URL.
- **Buttons** by position ([Design system](../design/design-system.md#buttons)), then `Open session`.

By kind:

| Request | Text / link | Actions |
|---|---|---|
| Review (PR on a forge) | `<Lead> wants to merge <PR title> into main.` Link: `PR #<n> on <Forgejo│GitHub│Gitea> · <branch> → main · checks <pass│fail│running>` | **`Open PR ↗`** (primary, the PR in a new tab), **`Send back`** (outline), **`Open session`**. No merge button: the CEO merges on the forge, and the request closes when Foreman sees the merge ([Pipeline and pull requests](./pipeline-and-prs.md#review-and-merge)) |
| Review (no forge) | `<Lead> wants <branch> merged into main in <repo>.` Link: `<branch> → main · checks <pass│fail│running>` | **`Send back`**, **`Open session`**. No merge button: the CEO merges with git, and the request closes when `main` contains the branch |

When the forge reports a PR as conflicting with `main`, its review panel's link line ends with **conflicts with main** (`--err`) instead of the checks; the lead has already been asked to rebase ([When main moves](./pipeline-and-prs.md#when-main-moves)).
| Approval, Question | the request | its options, plus an inline input `Or type an answer…` (flex 1, min 180px); Enter submits |
| Chief of Staff config review | `Chief of Staff changed <project>/.foreman/project.yaml: <change>.` Link: `foreman/config-N → main · 1 file changed`, as a PR or a branch like any review | **`Open PR ↗`** (with a forge), `Discard` (danger style, with a confirmation: closes the PR and deletes the branch), `Open Chief of Staff` |

**Send back** expands an inline editor under the request panel: a textarea (3 rows, placeholder `What needs to change before this can merge?`), then `Cancel` and the primary `Send to <Author>` (disabled while empty). Only one editor is open at a time; closing it clears the note. Effect: [Pipeline and pull requests](./pipeline-and-prs.md#review-and-merge).

**Empty**: `Nothing needs you on this project.`

## 2. In flight

Label "In flight" with the subtitle `worktree → lead → PR`. One panel listing this project's open pipeline entries, rows separated by a `--line` border ([Pipeline and pull requests](./pipeline-and-prs.md)):

**Work items** not in a PR ([Work item](../models/work-item.md)), rows (grid `auto 1fr auto auto`, padding `12px 16px`):

- Avatar 26 of the author.
- The title of the board card it implements (or the branch when it has none) (500), then `<Author> · <branch> · <note>` (branch in Mono; the note colored by its tone, derived from the work item's state: `ok` `--ok`, `err` `--err`, `ask` `--accTx`, otherwise `--tx3`).
- The stage chip: `in worktree`, `handed to lead`, `lead reviewing`, `integrated`.
- A **`Session`** button: opens the author's session.

**Open pull requests** ([Pull request](../models/pull-request.md)), after the work items:

- Avatar 26 of the lead, `PR #<n>: <title>` (or `<title>` without a forge) (500), then `<Lead> · <branch> → main · checks <status>` and the note.
- The stage chip `PR open`, and **`Open PR ↗`** (with a forge) or **`Session`**.
- Under it, indented one step, a compact line per work item it carries: Avatar 20 of the author, the board card title, the branch (Mono `--tx4`).

Entries of a stopped [deployment](../models/deployment.md) stay listed with a `stopped` chip (`--tx4`) instead of the Session button.

**Empty**: `No work in flight.`

## 3. Merged

Shown only when there is something. Pull requests merged in the last 7 days, newest first, at most 20: `✓` (`--ok`), `PR #<n>: <title>` and a note (`--tx4`), e.g. `Merged to main at 14:05`; under each, its work items as compact lines (author avatar, board card title).
