---
type: spec
status: partial
tags: [architecture, git, security, agents, pipeline]
---

# Sandboxes and git policy

Each [instance](../models/instance.md) works in its **own git worktree** of each code repo of the project, on its own branch, never in the CEO's checkout. Only a team lead's branches (`lead/<team>/*`) can be requested for merging into `main`, and only the CEO's action merges them. The [Git block](./building-blocks.md#8-git-git) creates the worktrees on the host; the [session runner](./session-runner.md) mounts them into the instance's container. **Git belongs to Foreman: agents only read it.** Every git write (a commit, a merge, a new branch) is a team tool that Foreman runs on the host ([below](#agents-and-git)). The project's knowledge base is different: one live checkout of the [project repo](../models/project-config.md), shared by every agent ([below](#project-knowledge-base)).

**Phasing:** worktrees and branches per agent arrive in [Terminal multiplexer](../../backlog/epics/terminal-multiplexer.md) (epic 1); the `switch`, `commit` and `merge` tools, and hand-offs that name a branch, in [Messaging and hand-offs](../../backlog/epics/messaging-handoffs.md) (they need the agent link). **Decided: in epic 1 agents can't commit**: the CEO drives each session by hand and looks at the uncommitted work in each worktree; integration detection, checks and pushes to the forge in the [Git epic](../../backlog/epics/git.md).

## Layout

- **Worktree:** `~/.foreman/sandboxes/<project>/<agent>/<repo>/` for each code repo of the [project](../models/project-config.md#the-project-record), created on the host with `git -C <repo path> worktree add`. The code repo itself is wherever the CEO keeps it; Foreman writes only in its `.git`. A worktree shares its repo's object store and refs: nothing is copied, and every branch any agent creates is at once visible to the others and to the CEO. Agent ids are unique across teams, so worktree paths never collide ([Team definition](../models/team-definition.md#schema)).
- `~/.foreman/sandboxes/<project>/<agent>/` is the instance's **sandbox**: its working directory, holding its worktrees and its view of the project repo (`project/`, mounted there). It lives under Foreman's home, outside every repo, so nothing has to be git-ignored, and git run in it never finds the [global repo](../models/home.md#the-global-repo) as an enclosing repo.
- The project repo `~/.foreman/projects/<name>/` never gets agent worktrees: it is the live, shared knowledge base.
- **A located repo:** when the CEO points a moved code repo at its new path (Locate, [Project config](../models/project-config.md#the-project-record)), each existing worktree follows it on its next start (`git worktree repair`). A repo missing or changed blocks starting the project's agents until then.
- **Two projects, one repo:** a code repo may be in several projects. Different teams never collide in it (their branches are named after agents and teams, all unique), but the same team can't be deployed on two projects that share a repo: its branches would be the same in that repo's `.git`, so the deploy is refused.

## Branches

- **Home branch** `wt/<agent>/main`: each member's own starting point (the lead's is `lead/<team>/integration`: the lead never has `wt/` branches) in a repo, created from `main` when its worktree is first made. Git lets a branch be checked out in only one worktree at a time, so no agent checks out `main` itself; members idle here.
- **Topic branch** `wt/<agent>/<topic>` (e.g. `wt/tove/LED-44`, the topic usually a card id): where a member does one piece of work. Agents can't create branches themselves: `switch(branch)` has Foreman switch the agent's worktrees to it, creating it from `main` the first time ([Agents and git](#agents-and-git)).
- **Lead branches** `lead/<team>/<topic>` (e.g. `lead/atlas/settlement-v2`): where the lead integrates hand-offs, any number of them, each reached (and made the first time) with `switch(branch)`; `lead/<team>/integration` is the default topic, created from `main` at deploy and checked out in the lead's worktree. They are the only source of merge requests to `main`, one open merge request per lead branch at a time. The team is part of the name because two teams can be deployed on the same project.
- `main` (name from `project.toml` `main`) changes **only by the CEO's decision**: Merge to main (Foreman merges on the CEO's click), a PR merged on the forge, or the CEO's own git; Foreman never merges on its own.

On deploy ([Agent runtime](./agent-runtime.md#instance-lifecycle)), `Git.Prepare` makes each missing worktree (an existing one is reused as it is). Since every worktree shares the repo, a new `main` is visible to all of them as soon as the repo has it.

**A deployment resumes; new work starts from `main`** (the CEO, 2026-10-22). Stop then Deploy brings the same team back to the same worktrees and branches, with the new goal; branch names are never salted per deployment, as topics already keep pieces of work apart, and `switch(<new topic>)` starts each new one from `main`. So that leftovers never get in the way:

- **Idle desks follow `main`.** A home branch holding nothing `main` lacks (merging it would change nothing), with nothing uncommitted where it is checked out, moves to `main` (`Git.Freshen`): whenever an agent's worktrees are prepared (deploy, wake, restart) and after every merge into `main` ([Pipeline and merges → Merge to main](../features/pipeline-and-merges.md#merge-to-main)). A desk with work stays as it is: the agent resumes it, or commits it to its topic and switches.
- **Work not wanted goes cleanly.** An agent drops what it didn't commit with `discard()`; the CEO drops a closed review's branches with Close ([Pipeline and merges → Review and merge](../features/pipeline-and-merges.md#review-and-merge)). Either way a copy is kept ([Backups](#backups)).

Stopping a deployment keeps the worktrees on disk; they are removed (`git worktree remove`) only on an explicit CEO action ([not built](../../backlog/remove-worktrees.md)). **Branches are cleaned up:** once a deployment ends, Foreman deletes each of its agents' `wt/*` branches that is merged into `main` (`Git.Prune`, [not built](../../backlog/after-merge-cleanup.md)); unmerged ones stay. A branch still checked out in a kept worktree can't be deleted, so it goes when that worktree is removed.

## Backups

What Foreman throws away, it keeps first, as a ref under `refs/foreman/<kind>/<time>/…` (time as `2026-10-22T09-30-00Z`, UTC): out of the branch list, restorable by hand, and deleted **30 days** after it was made (at Foreman's start, then hourly, and whenever a new one is made in that repo). A ref under `refs/foreman/` whose name carries no time is left alone.

| Kind | What | Made by |
|---|---|---|
| `closed/<time>/<review>/<branch>` | a dropped branch's head | Close, dropping its branches |
| `closed/<time>/<review>/<branch>@uncommitted` | what the worktree holding it hadn't committed (a merge not finished included), as a commit on its head | the same |
| `discarded/<time>/<agent>/<repo>` | what the agent hadn't committed, as a commit on its branch's head | `discard()` |

A copy of what wasn't committed is made through an index of its own (`GIT_INDEX_FILE`), so it records every file as it was (conflict markers included) and leaves the worktree's own index alone; files ignored by `.gitignore` aren't in it. A copy is never written over another: two made in the same second fail instead. To restore one: `git restore --source=<ref> -- .` in the worktree, or `git branch <name> <ref>`.

## Mounts

Everything at the **same path as on the host** ([Session runner](./session-runner.md#podman-engine-production)), except the project repo:

| Host path | In the container | Mode |
|---|---|---|
| `~/.foreman/sandboxes/<project>/<agent>/` (the sandbox: its worktrees) | same path; the working directory | rw |
| `<repo path>/.git` (each code repo, at its real path) | same path | **ro** |
| `~/.foreman/projects/<name>/` (the live project repo) | `<sandbox>/project` | rw for the lead, ro for the others |
| `~/.foreman/projects/<name>/.git` | `<sandbox>/project/.git`, over the mount above | **ro** for every agent |

A worktree's `.git` is a file pointing at `<repo path>/.git/worktrees/<name>` by host path, so git works inside the container only because the repo's `.git` is mounted at that same path. A submodule's `.git` file points into that same mount (`.git/worktrees/<name>/modules/<path>`), so it resolves there too ([Submodules](#submodules)). The CEO's working files (the code repo outside `.git`) are never mounted, and no agent ever touches the CEO's checkout or checks out a branch there. Foreman's own Merge to main runs in it only when it has `main` checked out, so its files move with `main` ([Pipeline and merges → Merge to main](../features/pipeline-and-merges.md#merge-to-main)).

## Agents and git

**Agents read git; Foreman writes it.** Every repo's `.git`, the project repo's included, is mounted read-only, so inside the container `git status`, `diff`, `log`, `blame` and `show` work, and every git write (`commit`, `checkout`, `branch`, `merge`, `rebase`, `push`) fails. Writes go through [team tools](./team-tools.md#tools), which Foreman runs on the host in the caller's own worktrees:

- **`switch(branch)`**: switches the agent's worktrees in each of its code repos to one of its own branches (`wt/<agent>/<topic>`, or `lead/<team>/<topic>` for the lead), creating it from `main` if it doesn't exist yet; so an agent can also go back to earlier work. A lead can keep several lead branches, and so several merge requests, at once.
- **`commit(commits)`**: for each repo the agent names, with its own message, stages everything changed there and commits it on the branch checked out there, authored as the agent (`<Name> <agent.invalid>`, author and committer: the CEO's identity never signs an agent's work). Repos not named keep their changes (the CEO, 2026-10-17). The lead may name `project`, the live project repo.
- **`merge(branch)`**: merges a named branch (the lead's, `main`, a teammate's) into the agent's worktrees. On a conflict the markers stay in the files; the agent resolves them and calls `commit`, which concludes the merge. Agents never rebase: branches are shared, and tracking follows commits.
- **`abort_merge()`**: backs out of an unfinished merge, branch and files as before it.
- **`discard(repos?)`**: throws away what the agent changed and didn't commit, in each repo it names (all when none): the files go back to the branch as committed, files not committed yet are removed (ignored ones stay). The changes are first kept ([Backups](#backups)). Refused during an unfinished merge (`abort_merge()` first). Agents can't do this with git: `git restore` and `git checkout -- <file>` write the index, which lives under the read-only `.git`.
- **`set_submodule(repo, path, ref, url?)`**: puts a submodule on a commit of its own repo, and adds it first with `url` ([below](#submodules)).
- **`handoff`** never commits: it names a committed branch, and refuses while the agent has uncommitted changes. `commit` is the one way to commit.

Foreman runs one git command at a time per repo, so git's own lock never collides. Its reads (the [Repositories](../features/repositories.md) tab's `status` of every worktree, refreshed on each agent git action) take no optional lock (`GIT_OPTIONAL_LOCKS=0`): they never write the index, so they can't hold `index.lock` while an agent's git needs it. And a git command cancelled midway (a browser request dropped) gets SIGTERM, on which git removes its lock files, and a kill only 5 s later; a kill straight away once left a stale `index.lock` in a lead's worktree, blocking its merges until the CEO deleted it (2026-10-18), and its git runs with the repo's hooks and fsmonitor turned off, so nothing configured in a repo runs a program on the host when Foreman acts. Its commits are unsigned, even where the CEO's git signs by default: they are automatic, and a signing agent would prompt, or fail, on every one. git's housekeeping after a commit (`maintenance`, `gc`) runs inside the command that triggered it, never detached: a detached one would escape the one-command-per-repo rule and could hold the repo's locks while Foreman's next command needs them (found 2026-10-20). The [session prompt](./session-prompt.md) tells agents that git writes go through the tools.

What this buys: no agent can damage refs, or plant a hook or a `.git/config` setting that would run a program on the CEO's machine the next time git runs there; every commit is authored as the agent who made it; and branch rules hold by construction, since Foreman is the only writer. **Trade-off, accepted:** every agent can still read every branch of the repo, its teammates' included.

**Later option, not built:** hide git from agents entirely, mounting only the worktree and no `.git`. Agents would no longer see other branches, at the cost of `diff`, `log` and `blame` inside the container, unless Foreman offered them as tools.

## Submodules

A worktree keeps its submodules' git data under its own git dir (`<repo>/.git/worktrees/<name>/modules/`), read-only in the container like the rest of `.git`, so agents can't fill a submodule or move it. Foreman does, on the host (the CEO, 2026-10-19: Foundry repos keep their dependencies as submodules under `lib/`):

- **In step with the branch.** After it creates a worktree, and after `switch`, `merge` and `abort_merge`, Foreman checks each submodule out at the commit the branch pins (`git submodule update --init`), fetching what's missing, and removes the folder of a submodule the branch no longer has, when nothing was changed in it (a plain checkout leaves it behind, and the next `commit` would add it back); its git data stays, so a switch back needs no fetch. Nested submodules only with `recursive_submodules = true` ([Project config](../models/project-config.md)). An agent starts without a submodule that can't be fetched, rather than not at all: the next switch or merge tries again and says so in its result.
- **Nothing in the CEO's repo config.** Each command gets the URLs of the branch's `.gitmodules` on its command line, and always checks out (a `.gitmodules` can ask for `merge`, `rebase` or `none`): the repo's `.git/config` is never written.
- **`set_submodule(repo, path, ref, url?)`** puts the submodule at `path` on `ref` (a branch, tag or commit of its repo), discarding what was changed in its files, and commits that on the caller's branch, authored as the caller; with `url`, it adds the submodule first (`.gitmodules` and the pin, in one commit). The change reaches `main` only through the CEO's merge. During a merge it only stages, settling a submodule conflict, and the agent's `commit` concludes the merge.
- **Changes inside a submodule** can't be committed (the repo stores only the commit a submodule is on): `switch`, `merge`, `handoff` and `request_merge` refuse while a submodule has changed files, whatever `.gitmodules` says to ignore, and name them; `set_submodule` discards them.
- **A submodule conflict** has no markers, so `commit` refuses while one is unsettled, rather than let `git add` settle it on whatever the folder is on.
- **A repo inside a worktree that isn't a submodule** (an agent's `git clone` or `git init`) is refused by `commit`: git would commit a pointer to a commit no one else has.
- **Fetches never carry the CEO's credentials**: no global or system git config (credential helpers, a keychain, URL rewrites), no credential helper of the repo, no SSH agent, no prompt, an empty home (no `.netrc`), and **https only**, for nested submodules too. An agent picks these URLs: with the CEO's credentials it could pull any private repo the CEO can reach into its sandbox, and from there out to the network. A public dependency comes; a private one is refused, and the agent asks the CEO. So do an `ssh` or scp-style URL and a URL relative to the repo's remote.

## Hand-offs

A hand-off is a message naming a branch: `handoff(mara, "wt/tove/LED-44", summary)`. Every worktree shares the repo, so the lead can review the branch at once (`git diff` in its own worktree) and merge it with `merge`; nothing is fetched or copied, and agents never push anywhere.

## Branch policy

| Who | May hand off or request a merge of |
|---|---|
| Member | `wt/<self>/*` only |
| Lead | `lead/<team>/*` only: the lead works only on its lead branches, never on `wt/` ones; it catches up with a newer `main` by merging it in (`merge("main")`), never by rewriting the branch |
| Nobody | `main`: it changes only by the CEO's merge |

### Where the policy is enforced

By Foreman, never by hooks in the repo: agents can't write `.git` at all, and Foreman is the only one that writes it, through the tools:

- **`switch`**, **`commit`**, **`merge`**, **`abort_merge`**, **`discard`** and **`set_submodule`** act only on the caller's own worktrees and the branches checked out there; **`handoff`** accepts only `wt/<self>/*`; **`request_merge`** only the caller's own `lead/<team>/*` ([Protocol → Team tools](./protocol.md#team-tools)).
- **Agents never push, and nothing is pushed on its own.** Only the CEO's **Create PR** on a merge request makes Foreman push the lead branch to the **forge** from the host, with the forge account's token ([Forge adapters](./forge-adapters.md)); agents never receive forge tokens. Calling `request_merge` again on a lead branch whose merge request is still open refreshes it (after merging `main` in, or new integrations): Foreman reruns the checks, and pushes again if it has a PR. Lead branches only ever move forward: nothing is rebased.
- **Merge to main** is Foreman merging the branch into `main` on the host, on the CEO's click; with or without a forge ([Pipeline and merges](../features/pipeline-and-merges.md#review-and-merge)).

## Project knowledge base

The [project repo](../models/project-config.md#the-project-repo) `~/.foreman/projects/<name>/` (`project.toml` and the wiki) is **one live checkout**, mounted into every container of the project: read-write for the **lead** (the lead of each team deployed on the project), read-only for everyone else, enforced by the mount mode. Its `.git` is read-only for every agent, the lead included: the lead edits the files and calls `commit()`, which commits on the project repo's `main` directly: no branches, no merge requests. Everyone else contributes through the [`contribute`](./team-tools.md#tools) team tool: Foreman writes the contribution into `project/wiki/inbox/` and tells the lead. There is no writable folder for read-only agents.

**Several leads, one checkout.** Each team deployed on the project has its own lead, and they all share this one checkout. A lead's `commit()` naming `project` takes everything changed in it, so it can sweep another lead's unfinished edit into its own commit: nothing is lost, only the attribution suffers. Accepted for v1; a worktree per lead, with Foreman merging each commit into `main`, is an option for later ([backlog](../../backlog/index.md#not-yet-scheduled)).

**Foreman commits its own writes** to the project repo, one commit per write, like the [global repo](../models/home.md#the-global-repo)'s one commit per save: a `contribute` note authored as the contributing agent, a checkbox tick or card move from the UI authored as the CEO. Each commit holds only the file Foreman wrote, so the lead's uncommitted edits in other files are left alone. When the file itself already has changes nobody committed (the lead mid-edit, or an entry an agent created and hasn't committed yet), Foreman writes it but doesn't commit it: the change goes into the lead's next commit, rather than the lead's edits being committed as the CEO's (the CEO, 2026-10-17).

**Changes to `project.toml` by the Chief of Staff** come later, and how they are approved is decided then ([Project config with the Chief of Staff](../../backlog/cos-project-config.md)). Nothing in the project repo is pushed on its own: the CEO pushes its `main` with **Sync** in Project settings ([Forge adapters → Sync](./forge-adapters.md#sync-the-reserved-repos)) ([Pipeline and merges](../features/pipeline-and-merges.md#review-and-merge)).

## Checks

`checks` from `project.toml` (e.g. `npm test`) run "in a sandbox before handoff and on a merge request". The container image must contain the project's toolchain for this to work ([Agent images](./tech-stack.md#agent-images): the project layer carries the toolchain).

Foreman runs them itself, in a **throwaway container** from the project's image on a clean checkout of the branch: before a handoff is accepted (the member's branch) and on `request_merge` (the lead branch). A clean checkout makes the result reproducible and immune to local state in a worktree. The result is the `checks_status` of the [work item](../models/work-item.md) (handoff) or of the [merge request](../models/merge-request.md) (`request_merge`); when a forge is configured, its CI statuses are shown alongside ([Forge adapters](./forge-adapters.md)).
