---
type: spec
status: partial
tags: [feature, ui, requests, pipeline]
---

# Project pipeline

What needs the CEO on a project, the work moving towards the [CEO's branches](./pipeline-and-merges.md#where-a-merge-goes), and what recently merged. These were the **Changes** tab until 2026-10-17, when the CEO split them by what they are ([Repositories card](../../backlog/archive/project-code-tab.md)):

- **Needs you** is in the project's [Inbox](./reviews.md#the-inbox), after its reviews, only when something waits; the Inbox's sidebar badge counts both (the CEO, 2026-10-31; above the wiki on Overview until then).
- **In flight** and **Merged** are in the project's [Team](./project-team.md) section, under its agents (on Repositories from 2026-10-17 to 2026-10-21, then Worktrees until 2026-10-31).

Each section keeps the project sections' 24px sides, with 36px between sections.

## 1. Needs you

This project's open [requests](../models/request.md), one **request panel** each, but its reviews: they come first in the same [Inbox](./reviews.md#the-inbox), and In flight's **Review** buttons open them (the CEO, 2026-10-21). Semantics of each kind: [Requests](./requests.md). Today only review requests are built, so the section doesn't show yet.

- **Request panel**: `--panel`, radius 12, padding `16px 18px`, gap 12.
- **Header**: the **kind chip first**, the agent name, the context (`<Team> on <project>`), a spacer, and the **source** (Mono 11.5px `--tx4`): `escalate(approval)`, `escalate(question)`, `escalate(finding)`, `request_merge()` or `raised by Foreman` (blocked), derived from the request's `kind` and `source` ([Request](../models/request.md)).
- **Text**: the summary, 15.5px.
- **Link line** (optional, Mono 12.5px `--accTx`), a real link where there is a URL.
- **Buttons** by position ([Design system](../design/design-system.md#buttons)), then `Open session`.

By kind:

| Request | Text / link | Actions |
|---|---|---|
| Approval, Question | the request | its options, plus an inline input `Or type an answer…` (flex 1, min 180px); Enter submits |

**Empty**: the section is left out.

## 2. In flight

Label "In flight" with the subtitle `worktree → lead → target`. One panel listing this project's open pipeline entries, rows separated by a `--line` border ([Pipeline and merges](./pipeline-and-merges.md)):

**Work items** not in a merge request ([Work item](../models/work-item.md)), rows (grid `1fr auto auto`, padding `12px 16px`):

- The title of the board card it implements (or the branch when it has none) (500), then `<Author> · <branch> · <note>` (branch in Mono; the note colored by its tone, derived from the work item's state: `ok` `--ok`, `err` `--err`, `ask` `--accTx`, otherwise `--tx3`).
- The stage chip: `in worktree`, `handed to lead`, `lead reviewing`, `integrated`.
- A **`Session`** button: opens the author's session.

**Open merge requests** ([Merge request](../models/merge-request.md)), after the work items:

- Its review's title (the lead's) (500), then `<Lead> · <repo> · <branch> → <target>`, `· checks <status>` once checks have run, and the note (`--err` when it conflicts). `<target>` is where that merge request goes: the repo's default branch unless the lead named another branch, here and on every Merge button.
- The stage chip `merge requested`, **`Review`** (opens its [review](./reviews.md)), and **`Session`**.
- Under it, a compact line per work item it carries (indented 16px): the board card title, the branch (Mono `--tx4`), the author's name (`--tx4`).

Entries of a stopped [deployment](../models/deployment.md) stay listed with a `stopped` chip (`--tx4`) instead of the Session button.

**Empty**: `No work in flight.`

## 3. Merged

Shown only when there is something. Merge requests merged in the last 7 days, newest first, at most 20: `✓` (`--ok`), its review's title (with `PR #<n>` when it went through one; the branch instead when that review was decided before the Inbox's last 20, which carry the titles) and a note (`--tx4`), e.g. `api · merged to main · 14:05` (its repo and the target it merged into; local time, with the date when not today); under each, its work items as the same compact lines.
