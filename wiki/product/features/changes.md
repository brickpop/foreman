---
type: spec
status: planned
tags: [feature, ui, requests, pipeline]
---

# Changes

The project's actionable tab: what needs the CEO on this project, the work moving towards `main`, and what recently merged. Body padding `28px 40px 64px`, max width 860px, 36px between sections. The project header and tabs are described in [Wiki reader](./wiki-reader.md#project-header-and-tabs).

## 1. Needs you

This project's open [requests](../models/request.md) as cards. Semantics of each kind: [Requests](./requests.md).

- **Card**: `--panel`, radius 12, padding `16px 18px`, gap 12.
- **Header**: the **kind chip first**, Avatar 24 (no ring), the agent name, the context (`<Team> on <project>`), a spacer, and the **source** (Mono 11.5px `--tx4`): `escalate(approval)`, `escalate(question)`, `escalate(finding)`, `open_pr()`, `raised by Foreman` (blocked) or `Chief of Staff`.
- **Text**: the summary, 15.5px.
- **Link line** (optional, Mono 12.5px `--accTx`), a real link where there is a URL.
- **Buttons** by position ([Design system](../design/design-system.md#buttons)), then `Open session`.

Kind-specific cards:

| Card | Text / link | Actions |
|---|---|---|
| Review (PR) | `<Author> wants to merge <card> into main.` Link: `Open on <Forgejo│GitHub│Gitea> ↗ · <branch> → main · checks <pass│fail│running>`, or `Local PR · <branch> → main · checks …` when the forge is `none` | **`Merge`** (primary), **`Send back`** (outline), **`Open session`** |
| Question | the question | its options, plus an inline input `Or type an answer…` (flex 1, min 180px); Enter submits |
| Chief of Staff config review | `Chief of Staff changed <project>/.foreman/project.yaml: <change>.` Link: `foreman/config-N → main · 1 file changed` | `Merge`, `Discard`, `Open Chief of Staff` |

**Send back** expands an inline editor under the card: a textarea (3 rows, placeholder `What needs to change before this can merge?`), then `Cancel` and the primary `Send to <Author>` (disabled while empty). Only one editor is open at a time; closing it clears the note. Effect: [Pipeline and pull requests](./pipeline-and-prs.md#ceo-actions).

**Empty**: `Nothing needs you on this project.`

## 2. In flight

Label "In flight" with the subtitle `worktree → lead → PR`. A card list of this project's [pipeline items](../models/pipeline-item.md) at stages 0–3; rows (grid `auto 1fr auto auto`, padding `12px 16px`):

- Avatar 26 of the author.
- The card title (500), then `<Author> · <branch> · <note>` (branch in Mono; the note colored by its tone: `ok` `--ok`, `err` `--err`, `ask` `--accTx`, otherwise `--tx3`).
- The stage chip: `in worktree`, `handed to lead`, `lead reviewing`, `integrated`.
- A **`Session`** button: opens the author's session.

**Empty**: `No work in flight.`

## 3. Merged

Shown only when there is something. Recently merged items: `✓` (`--ok`), the card title, and a note (`--tx4`), e.g. `Merged to main at 14:05`.

## Open questions

1. **How long "Merged" lists items.** Options: last N (e.g. 10); last 7 days; since the CEO last opened the tab. Recommendation: last 7 days, max 20.
