---
type: task
status: done
priority: high
tags: [feature]
epic: git
---

# Resume, clean desks, discard and backups

**Why:** a deployment is a standing pair (team + project): Stop then Deploy resumes it, same worktrees, branches and work items, with the new goal. What's left from earlier work must never get in the way of starting something new from `main`, and nothing an agent or the CEO drops may be lost (the CEO, 2026-10-22).

## Decided (the CEO, 2026-10-22)

- **No salted branch names.** Topic branches already keep pieces of work apart; new work starts from `main` with `switch(<new topic>)`.
- **Idle desks follow main.** A home branch (`wt/<agent>/main`, `lead/<team>/integration`) with nothing unmerged and nothing uncommitted moves to the current `main`: on deploy, and after a merge. A desk with work stays as it is.
- **`discard(repos?)`**, a team tool: the agent's worktrees back to their branch as committed (changes reverted, new files removed, ignored ones kept); the changes are first saved as a commit under `refs/foreman/discarded/…`. Refused during an unfinished merge (`abort_merge()` first). Agents can't do it with git: the index is under the read-only `.git` (`git restore` fails).
- **Close dropping a branch an idle worktree holds:** its changes not committed are saved beside the branch's backup, then the worktree is reset to `main`: no more "kept" branches for changes in the way.
- **Stop is unchanged:** worktrees and branches stay. Removing worktrees is its own explicit action: [Remove a stopped deployment's worktrees](../remove-worktrees.md).
- **Backups expire:** every saved copy under `refs/foreman/` carries its date in its name and is deleted 30 days later.
- **A resumed agent is told what it has:** a notice when its session starts fresh (the session prompt stays the same for every run: live git state would go stale in it), listing its branches with work main lacks, its changes not committed and, for the lead, its open reviews. The prompt says, always, that worktrees persist and to check where you are (the CEO, 2026-10-22).

## Steps

- [x] git: save a worktree's changes as a commit; discard; backups named with their date; expiry (`internal/git/backups.go`)
- [x] git: Prepare moves an idle home branch to main (`Git.Freshen`; `git.HomeBranch` names it once)
- [x] workflow: `discard` tool; Close saves and resets; idle desks after a merge; `PruneBackups` at start and hourly
- [x] what a resumed agent is told: `Workflow.Resumed` (the manager's `Started` hook), and the prompt's `{worktrees persist}` paragraph
- [x] specs: sandboxes-and-git (resume model, Backups), team-tools, protocol, session-prompt, pipeline-and-merges, reviews, api, agent-runtime
