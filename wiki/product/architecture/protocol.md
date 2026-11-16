---
type: spec
status: planned
tags: [architecture, team-tools, requests, pipeline]
---

# Protocol

The messaging and workflow schema: who sends what, through which [building block](./building-blocks.md), as which struct, with which lifecycle, and what the UI hears. The models stay the single source of the **stored** fields ([Team message](../models/team-message.md), [Request](../models/request.md), [Work item](../models/work-item.md), [Merge request](../models/merge-request.md), [Instance](../models/instance.md)); this entry owns the wire and in-memory types and the interactions between blocks. Go structs are sketches: field names and comments are the contract, exact types are settled in code.

## Actors and channels

Three actors, and every interaction enters through exactly one channel:

| Actor | Channel | Struct | Received by |
|---|---|---|---|
| Agent | A team-tool call: harness → `foreman-agent mcp` → agent port | [`ToolCall`](#envelopes) | Agent link → **Messaging** (`ask`, `reply`) or **Workflow** (`open`, `switch`, `commit`, `merge`, `abort_merge`, `handoff`, `contribute`, `escalate`, `fetch`, `request_merge`, `withdraw_merge`, `reviews`) |
| Agent | A harness hook: `foreman-agent hook <event>` → agent port | [`HookEvent`](#envelopes) | Agent link → **Instance manager** (status), which tells Messaging and Workflow through events |
| CEO | Keystrokes in a terminal pane | raw bytes | Web server → **Terminal** `Input` (no Foreman logic) |
| CEO | Line input (`POST /api/instances/:id/line`) | [`CEOInput`](#envelopes) | Web server → **Terminal** `Input`, as the same bytes typing would produce |
| CEO | Answering a request (Dashboard, project Inbox) | `Answer` | Web server → **Workflow** `Resolve` |
| CEO | On a [review](../models/review.md): comments, Request changes, Merge (the repos ticked), Dismiss | `ReviewAction` | Web server → **Workflow** |
| Foreman | Process exit, silence, rate limit, git or build error | internal | **Instance manager** / **Workflow** → blocked [request](#requests) |
| Foreman | Target poll (a merge outside Foreman), HQ watch | internal | **Workflow** → [merge request](#work-items-and-merge-requests) transitions |

Agents never reach each other except through Messaging, never reach the CEO except through Workflow, and never push.

## Envelopes

```go
// ToolCall: one team-tool call, already authenticated by the agent link.
type ToolCall struct {
    CallID     string          // the MCP call id; the result is matched to it
    Instance   string          // "mara@ledger": from the token, never from the payload
    Deployment string          // the instance's deployment: from the token too
    Tool       string          // "ask" | "reply" | "switch" | "commit" | "merge" | "abort_merge" | "discard" | "set_submodule" | "handoff" | "contribute" | "escalate" | "fetch" | "request_merge" | "withdraw_merge" | "reviews"
    Args       json.RawMessage // the tool's args struct, below
    At         time.Time
}

// ToolResult: what the tool call returns to the harness.
type ToolResult struct {
    CallID string
    OK     bool
    Value  json.RawMessage // the tool's result struct, below
    Error  *ToolError      // set when !OK
}
type ToolError struct {
    Code    string // a typed code from the tool's table below; the agent reads Message
    Message string // one human sentence, e.g. "No teammate \"rhea\" on ledger. Teammates: mara, tove."
}

// HookEvent: a harness hook, normalised by the harness adapter.
type HookEvent struct {
    Instance string // from the token
    Event    string // normalised: "started" | "busy" | "idle" | "waiting" | "ended"
    Raw      string // the harness's own event name, e.g. Claude Code "Stop", "Notification"
    Data     json.RawMessage // the harness's payload, kept as-is for diagnosis
    At       time.Time
}

// CEOInput: line input from the UI (mobile, paste). Raw keystrokes don't use it.
type CEOInput struct {
    Instance string // always one instance: there is no broadcast to a deployment (talk to the lead)
    Text     string // written followed by Enter, exactly as typing it would be
}
```

The harness adapter's `Interpret` maps each harness's hooks to the normalised `Event`:

| Normalised | Meaning | Claude Code hook (example) | Instance status |
|---|---|---|---|
| `started` | The harness is up and at its prompt | `SessionStart` | `starting` → `idle` |
| `busy` | A turn started (it accepted input) | `UserPromptSubmit` | `running` |
| `idle` | The turn ended; it waits at its prompt | `Stop` | `idle`, or `done` right after a handoff ([Instance](../models/instance.md#statuses)) |
| `waiting` | It waits **for the user**: a permission dialog, a multiple-choice question, or a question it asked in plain text before stopping | `Notification` | `awaiting`, with a request on the Dashboard ([Waiting for the CEO](#waiting-for-the-ceo)) |
| `ended` | The harness is exiting | `SessionEnd` | (the Terminal's exit sets `stopped`) |

`awaiting` means its harness waits for the user (a `waiting` hook): typing into it would answer its dialog, so nothing is delivered meanwhile. An open `escalate(approval)` doesn't make the lead `awaiting`: it keeps its status, and its messages (the CEO, 2026-11-04); its session tab shows that it waits on the CEO ([Sessions](../features/sessions.md)). `idle` and `waiting` must never be confused: an instance merely idle after finishing raises nothing ([Harness adapters → Idle and waiting detection](./harness-adapters.md#idle-and-waiting-detection)).

### Waiting for the CEO

Whenever the harness reports it is waiting for the user, any agent, lead or member, the instance manager raises a request on the Dashboard and the project's Inbox (`internal/instances/waiting.go`):

- **Kind:** `question`, `Source: harness` (the panel reads "waiting in its session"), no options.
- **Content:** what the harness says it asks (its adapter's `Asks`): Claude Code's `AskUserQuestion` questions, each with its choices (`Which database? (Postgres / SQLite)`), or a permission prompt's message; Pi's dialog title. Otherwise `<Name> is waiting for your input.` Its first line is the title, cut to 120 characters with `…`; when it had more lines or was cut, the whole text is the body.
- **One per wait:** the hooks that repeat it raise no second.
- **Answer:** in the session, where the panel's **Open session** leads. Its next activity (`busy` or `idle`) resolves the request (`answered in its session`), as does its run ending. Typing the CEO's pick into the dialog from the panel is not built: it needs each harness's dialog keys checked live ([Waiting in its session](../../backlog/harness-waiting-requests.md)).
- After a Foreman restart, an agent still `awaiting` in its re-attached session keeps its request; any other's resolves (`Foreman restarted`).
- The instance is `awaiting` while it waits.

## Text into a session: deliveries and markers

Anything Foreman writes into a session's terminal is **typed input**: harnesses are full-screen programs that read the terminal as their keyboard. So there are two different things, never mixed:

- A **delivery** is text the agent must read: a teammate's ask, a reply, a handoff to the lead, the CEO's answer, a "target moved" notice. Messaging hands it to `Instances.Deliver`, which waits until the recipient is `idle` and then calls the harness adapter's `Deliver` (type the text, then submit). It starts a turn. **A delivery is always one line of plain words, with no symbols** (the CEO's decisions, 2026-10-15): a multi-line paste reaches Claude Code wrapped as pasted content, which it treats as possibly not the user's ([experiment D](../../backlog/archive/prove-d-claude-delivery-hooks.md)). A message's notice names it and quotes its start; the agent reads the whole message with `read_message`. Foreman's own notices are short sentences that fit the line.
- A **marker** is a note for the CEO's eyes only: "Mara asked Rhea (msg-41)", "Merge requested: lead/ledger/atlas/integration". It is a UI event (`instance.marker`) meant for the project's [Activity](../features/project-activity.md#the-timeline) (not shown yet), apart from the terminal. It is **never** written into the PTY, where it would corrupt the program's screen or be read as an instruction.

Every delivery is also shown as a marker. Delivered text is untrusted data: it is framed with its sender and capped ([Team tools → Delivery](./team-tools.md#delivery)).

## Team tools

Who may call each tool, how it blocks, which block handles it, and what each side sees. Role rules come from the [team definition](../models/team-definition.md); a tool not allowed to a role isn't listed in its MCP tool list at all.

| Tool | Who | Blocks the caller | Handled by |
|---|---|---|---|
| `ask` | anyone | no: returns the request id at once | Messaging |
| `reply` | the agent an `ask` was addressed to | no | Messaging |
| `read_message` | anyone, for a message to it | no | Messaging |
| `switch` | anyone | until the worktrees are switched (seconds) | Workflow (+ Git) |
| `commit` | anyone | until the commit is made (seconds) | Workflow (+ Git) |
| `merge` | anyone | until the merge is done (seconds) | Workflow (+ Git) |
| `abort_merge` | anyone | until the merge is backed out (seconds) | Workflow (+ Git) |
| `discard` | anyone | until the worktrees are clean (seconds) | Workflow (+ Git) |
| `set_submodule` | anyone | until the submodule is fetched and committed (seconds; longer for a large first fetch) | Workflow (+ Git) |
| `handoff` | members (the lead has no `wt/` branch to hand off) | no: returns once the branch is checked; checks run after | Workflow (+ Messaging for the delivery) |
| `contribute` | anyone | no: returns once the note is written | Workflow (+ Messaging for the delivery) |
| `escalate` | lead only | no: returns the request id at once; for an `approval` (or a `question` with `wait: true`) the lead waits for the answer before that step | Workflow |
| `fetch` | lead only | until every remote is fetched (seconds; at most 2 min a repo) | Workflow (+ Git) |
| `prune` | anyone | until the branches are deleted (seconds) | Workflow (+ Git) |
| `request_merge` | the lead only | no: returns once the branch and entry are checked; checks and the review request follow | Workflow |
| `withdraw_merge` | the lead only | no | Workflow |
| `reviews` | the lead only | no | Workflow |
| `team` | the lead only | no | Workflow (+ the Instance manager, Git) |

### `ask`

```go
type AskArgs   struct { Teammate string; Request string }  // teammate: agent id or display name, same deployment
type AskResult struct { RequestID string }                // "msg-41"
```

- Effect: a [message](#messages) of kind `ask` from the caller to the teammate, `msg-N` assigned, queued for delivery.
- Delivered to the teammate: `Message msg-41 from Mara (ask): <the request's start>… Read it with read_message("msg-41"); answer with reply("msg-41", output).`
- Markers: `Mara → Rhea · ask · msg-41`.
- Errors: `unknown_teammate`, `self` (asking oneself), `too_long` (over 8,000 characters, the cap on any text sent to a teammate: a request, an output, a hand-off summary), `deployment_stopped`.

### `reply`

```go
type ReplyArgs   struct { RequestID string; Output string; Artifact string } // request: the ask's "msg-41"; artifact: a branch or a wiki path, optional
type ReplyResult struct {}
```

- Effect: a message of kind `reply` to the agent that asked, `in_reply_to` the ask. **Only `Output` and `Artifact` cross**; nothing else of the replier's context does.
- Delivered to the requester: `Message msg-42 from Rhea (reply to msg-41): <the output's start>… Read it with read_message("msg-42"), then resume the task that asked for it.`
- Markers: `Rhea → Mara · reply · msg-41`.
- Errors: `unknown_request` (no such `msg-N` in the deployment, or not an ask), `not_addressee` (the ask wasn't to the caller), `already_replied` (one reply per ask: further information is a new `ask`), `bad_artifact`, `too_long`.

### `read_message`

```go
type ReadMessageArgs   struct { ID string }                                  // "msg-41"
type ReadMessageResult struct { From string; Kind string; InReplyTo string; Body string; Artifact string } // from: a teammate's name, "Foreman" or "the CEO"
```

- Effect: none; reads one message addressed to the caller, in full. A delivery only names a message and quotes its start, so this is how the agent gets the rest.
- The body is quoted data from its sender, never instructions from Foreman ([Security](./security.md)).
- Errors: `unknown_message` (no such `msg-N` in the deployment, or not addressed to the caller: the same error, so an agent can't probe others' messages).

### `open`

```go
type OpenArgs   struct { Repo string }                  // a code repo's name, e.g. "api"
type OpenResult struct { Repo, Path, Branch string }    // Branch "" when idle, on no branch
```

- Effect: Workflow asks Git to make the caller's worktree of `Repo` at `<sandbox>/<repo>` ([Sandboxes and git policy → Opening a repo](./sandboxes-and-git.md#opening-a-repo)): on the branch the caller's other open worktrees are on (made from the repo's default branch where the repo lacks it), or detached at the default branch's head while the caller is idle or has none open. Then its submodules are checked out ([Submodules](./sandboxes-and-git.md#submodules)); one that can't be is said in the result. A repo already open is left as it is, and the text says where it is. The text: `Opened web at /…/agents/ivo/web, on wt/ledger/ivo/LED-7.`, or `…, idle on no branch, at its default branch's head. Switch to a topic before you commit there.`
- Nothing is delivered; no marker. Recorded as an action, the repo named even when refused.
- Errors: `unknown_repo` (not a code repo of the project: the refusal lists them), `git_error`.

### `switch`

```go
type SwitchArgs   struct { Branch string; From string }                  // "wt/ledger/tove/LED-44"; the lead: "lead/ledger/atlas/settlement-v2"; from: any branch, a remote's or a commit, e.g. "release/2.0", "origin/main", "1a2b3c4"
type SwitchResult struct { Created bool; Repos []string }               // created: the branch was new (made from From, or the default branch); the repos switched
```

- Effect: Workflow asks Git to switch the caller's open worktrees to `Branch`, and opens on it the caller's repos not open yet that have `Branch` (an earlier topic's),  creating it where it doesn't exist yet from `From` in each repo that has it, and from the repo's default branch where it hasn't or without `From` ([Where a merge goes](../features/pipeline-and-merges.md#where-a-merge-goes)). `From` is any branch (the CEO's, a teammate's, the lead's), a remote's branch (`origin/main`, after [`fetch`](#fetch)) or a commit; a name is resolved as a branch first, over a tag of that name. The new branch is made at that commit, so no upstream is set. `Branch` must be one of the caller's own: `wt/<project>/<agent>/<topic>` for a member, `lead/<project>/<team>/<topic>` for the lead (the lead never works on `wt/` branches). The same call starts new work and goes back to earlier work (e.g. a branch whose merge request was rejected); on a branch that exists, `From` changes nothing and the text says so. A lead can keep several lead branches, and so several merge requests, at once. Agents can't create branches themselves: each repo's `.git` is read-only ([Sandboxes and git policy → Agents and git](./sandboxes-and-git.md#agents-and-git)). From no branch (idle) to a new branch without `From`, it is made where the worktree is, keeping its uncommitted changes (`New in api (where it was idle, changes kept)`): an agent may start editing before it names its topic. The text says `Switched api, web to wt/ledger/tove/LED-44. New in api (from release/2.0), web (from trunk).`, then `Opened docs, which had it.` when it opened any, and `Not open: infra: open(repo) any your work needs.` while some repo isn't open.
- Nothing is delivered; no marker.
- Errors: `no_repos` (no repo open, and none has `Branch`: `open` the repos the work is in first), `submodule_changed` (as for `merge`), `uncommitted_changes` (commit first, so no work is carried from one branch to another; not from no branch to a new one without `From`), `branch_not_allowed` (not one of the caller's own), `bad_branch` (not a valid branch name), `no_such_ref` (none of the caller's repos has `From`: "name a branch, a remote's branch (fetch() first) or a commit"), `git_error`.

### `commit`

```go
type CommitArgs   struct { Commits []RepoMessage }       // at least one; each repo once
type RepoMessage  struct { Repo string; Message string }
type CommitResult struct { Commits []RepoCommit }        // one per repo named that had changes
type RepoCommit   struct { Repo string; Branch string; Commit string }
```

- Effect: for each repo named, Workflow asks Git to stage everything changed in it and commit it, with that repo's message, on the branch checked out there, authored as the agent (from its token). A repo is the caller's worktree of that code repo (its folder name in the sandbox) or, for the lead only, `hq`: the project's HQ, committed on its `main` (other agents can't write that checkout; `hq` is reserved, so no code repo takes that name). Repos not named keep their changes (the CEO, 2026-10-17: the lead commits the wiki without its half-done code, and a commit's message fits its repo). Every entry is checked before anything is committed. A repo named with nothing changed is said in the result (`Nothing changed in web.`), not refused, unless no repo had changes. Agents can't commit themselves: every repo's `.git`, the HQ's included, is mounted read-only ([Sandboxes and git policy → Agents and git](./sandboxes-and-git.md#agents-and-git)). After a `merge` with conflicts, committing a repo concludes its merge. A commit carries its message alone: the attribution a harness may add on its own is dropped first, every line starting `Co-Authored-By:` and Claude Code's `🤖 Generated with [Claude Code](…)` line, in any case (the CEO, 2026-10-29).
- Nothing is delivered; no marker.
- Errors: `empty` (no repo named, or a repo without a message once its attribution is dropped), `not_open` (a code repo the caller hasn't opened: nothing changed there), `unknown_repo` (not one of the caller's: the refusal lists them), `repeated_repo`, `no_branch` (a code repo named is on no branch, idle: `switch` to a topic first, which keeps the changes), `nothing_to_commit` (in none of the repos named), `submodule_conflict` (a submodule an unfinished merge left on no commit: settle it with `set_submodule`), `nested_repo` (a git repo inside the worktree that isn't a submodule: delete it, or add it with `set_submodule`), `conflicts_left` (conflict markers still in a file of an unfinished merge), `git_error` (with git's message). With the first three of these refusals, the repos without them are committed meanwhile.

### `merge`

```go
type MergeArgs   struct { Branch string }               // e.g. "lead/ledger/atlas/integration", "main", "wt/ledger/tove/LED-44"
type MergeResult struct { Repos []RepoMerge }
type RepoMerge   struct { Repo string; Merged bool; Conflicts []string } // files left with conflict markers
```

- Effect: Workflow asks Git to merge `Branch` into the branch checked out in each of the caller's worktrees whose repo has it, opening on the caller's branch those of its repos that have it and aren't open yet. Any branch of the repo may be merged in (reading is open to everyone), and a remote-tracking one the lead's [`fetch`](#fetch) brought (`origin/main`) where the repo has no local branch of that name: it is named to git in full (`refs/remotes/origin/main`), never left to git's guess. Only the caller's own worktrees change. On a conflict the markers stay in the files: the agent resolves them and commits those repos, which finishes the merge (it refuses while markers are left), or backs out with `abort_merge`. Agents never rebase: branches are shared from their first commit (a teammate may already have merged one), and integration detection and the CEO's own PRs follow commits, which a rebase would replace (decided 2026-10-16).
- Nothing is delivered; a merge into a lead branch may let Workflow detect an integration ([Work items and merge requests](#work-items-and-merge-requests)).
- Errors: `branch_not_found` (in none of the project's code repos), `no_branch` (the caller is idle, on no branch: `switch` to a topic first; nothing is opened), `submodule_changed` (files changed in a submodule, which no commit records: restore them, or `set_submodule`), `uncommitted_changes` (commit first, so a merge never mixes with unsaved work), `git_error`.
- After it, the submodules follow the branch ([Sandboxes and git policy → Submodules](./sandboxes-and-git.md#submodules)); a submodule that can't be updated is said in the result, the merge stands. A submodule conflict has no markers: `set_submodule` settles it.
- `merge` was called `update` until 2026-10-16: the git word, which agents already know.

### `abort_merge`

```go
type AbortMergeArgs   struct {}
type AbortMergeResult struct { Repos []string } // where a merge was backed out
```

- Effect: Workflow asks Git to back out of the unfinished merge in each of the caller's worktrees that has one (`git merge --abort`): the branch and its files are as they were before the merge. A repo whose merge finished cleanly keeps it.
- Nothing is delivered; no marker.
- Errors: `no_merge` (no merge under way in the caller's repos), `git_error`.

### `discard`

```go
type DiscardArgs   struct { Repos []string } // the caller's repos, by folder; all when empty
type DiscardResult struct { Repos []string } // where something was thrown away
```

- Effect: in each repo named (every one of the caller's worktrees when none), Workflow asks Git to keep what isn't committed as a commit on the branch's head under `refs/foreman/discarded/<time>/<agent>/<repo>`, then to put the worktree back to its branch as committed (`git reset --hard`, `git clean -d --force`: ignored files stay) ([Sandboxes and git policy → Backups](./sandboxes-and-git.md#backups)). Then the submodules follow the branch.
- Every repo is checked first, so it never stops halfway. A repo with nothing to throw away is left alone; with nothing anywhere, the result says so (no error).
- Nothing is delivered; no marker. Kept as a [team action](#team-actions) with the repos it cleaned.
- Errors: `unknown_repo`, `repeated_repo`, `merge_under_way` (an unfinished merge in one of them: `abort_merge()` first), `no_repos`, `git_error`.

### `set_submodule`

```go
type SetSubmoduleArgs   struct { Repo, Path, Ref string; URL string } // URL only to add a submodule
type SetSubmoduleResult struct { Repo, Branch, Commit string }         // Commit "" during a merge: staged only
```

- Effect: in the caller's worktree of `Repo` (opened first, on the caller's branch, when it isn't yet), Workflow asks Git to put the submodule at `Path` on `Ref` (a branch of its repo, a tag, or a commit, fetched by its id when no branch or tag reaches it), discarding what was changed in its files, and to commit that on the branch checked out there, authored as the caller (`Move submodule lib/forge-std to v1.9.0`). With `URL` it first adds the submodule: a clone into the worktree's own git dir, then `.gitmodules` and the pin, in one commit (`Add submodule lib/forge-std at v1.9.0 (<url>)`). During a merge it only stages: that settles a submodule conflict, and the caller's `commit` concludes the merge. Already on that commit, it only discards the changed files and says so, without a commit. Fetches go without the CEO's credentials, over https only, or come from the CEO's own checkout ([Sandboxes and git policy → Submodules](./sandboxes-and-git.md#submodules)).
- Nothing is delivered; no marker.
- Errors: `unknown_repo` (not one of the caller's code repos: the HQ has no submodules), `empty` (no path or no ref), `no_branch` (the caller is idle, on no branch: `switch` first), `bad_path` (not a path inside the repo), `not_a_submodule` (nothing at `Path` to move: give `URL` to add one), `path_taken` (adding over a submodule or files), `bad_url` (not https), `unknown_ref` (no such branch, tag or commit in the submodule's repo), `needs_credentials` (a private repo: the CEO adds it), `git_error`.

### `handoff`

```go
type HandoffArgs   struct { Teammate string; Branch string; Summary string } // branch: wt/<project>/<caller>/<topic>
type HandoffResult struct { WorkItem string; Checks string }                  // checks: "running"
```

- Effect: a hand-off is an ask that Foreman understands: it names a committed branch, so Foreman can track it. It never commits: committing has one way, `commit` (decided 2026-10-16), and a caller with uncommitted changes is refused, so what is handed off is exactly what was committed. Workflow checks `Branch` against the policy (a member may hand off only `wt/<project>/<self>/*`; [Sandboxes and git policy](./sandboxes-and-git.md#branch-policy)). Every worktree shares the repo, so nothing is fetched: the lead sees the branch at once. Workflow then runs the project checks in a throwaway container ([Checks](./sandboxes-and-git.md#checks)), creating or advancing the [work item](#work-items-and-merge-requests).
- Checks pass: the hand-off is delivered to the teammate: `Message msg-43 from Tove (handoff of wt/ledger/tove/LED-44): <the summary's start>… Read it with read_message("msg-43").` The caller gets `From Foreman: checks pass, wt/ledger/tove/LED-44 was handed to Mara.` as a delivery when idle.
- Checks fail: nothing goes to the teammate; the caller gets `From Foreman: checks fail on wt/ledger/tove/LED-44: <first line>. Fix it and hand off again.`
- Markers: `Tove → Mara · handoff · wt/ledger/tove/LED-44`, then the checks outcome.
- Errors (returned at once): `unknown_teammate`, `branch_not_allowed` (not `wt/<project>/<self>/*`), `submodule_changed` (as for `merge`), `uncommitted_changes` (commit first), `branch_not_found`, `nothing_new` ("<branch> changes nothing that its repos' default branch, or a branch merges went into, doesn't have": it has [landed](../features/pipeline-and-merges.md#merge-to-a-target) in each repo).
- Phasing: since the [Git](../../backlog/epics/git.md) epic's first card (2026-10-17) a hand-off creates or advances its work item; the checks come with the next card, `git-checks`. Until then a hand-off goes to "handed to lead" at once, with no checks.

### `contribute`

```go
type ContributeArgs   struct { Kind string; Title string; Body string } // kind: "finding" | "proposal" | "note"
type ContributeResult struct { Path string }                            // "<hq>/wiki/inbox/2026-10-14-tove-flaky-settle-test.md": on disk, the same path in the lead's container
```

- Effect: Workflow writes a new entry `wiki/inbox/<yyyy-mm-dd>-<agent>-<slug>.md` into the project's HQ, the slug made from `Title`. Its frontmatter: `type: note`, `kind`, `author` (the caller's agent id, from its token, never from the arguments) and `deployment`; its body is `# <Title>`, then `Body`. A title is one line (at most 120 characters), so nothing in it can reach the frontmatter. A body is at most 32,000 characters: it becomes a file, not a line in the lead's conversation, so a whole design record fits (the CEO, 2026-10-19; messages keep 8,000). Limits count characters, not bytes. A taken file name gets `-2`, `-3`. The commit holds only that file, authored as the agent, so the lead's unfinished edits in the same checkout stay out of it. Agents without write access to the knowledge base contribute only this way: there is no writable folder for them ([Sandboxes and git policy](./sandboxes-and-git.md#project-knowledge-base)).
- Delivered to the caller's lead: `Message msg-44 from Tove (contribution): <title>, in <hq>/wiki/inbox/<file>.` The lead decides what becomes of it.
- Markers: `Tove → Mara · contribute · msg-44`, like every other kind; the project's Activity shows the title.
- Errors: `bad_kind`, `empty` (no title or no body), `too_long`.

### `escalate`

```go
type EscalateArgs struct {
    Kind    string   // "approval" | "question" | "finding"
    Title   string   // required: one line, at most 120 characters; what the CEO reads first
    Body    string   // optional: the detail (context, what was tried, the trade-offs), at most 8,000 characters
    Options []string // optional, at most 6 distinct one-line labels of 80; the first is the primary action. An approval without them gets Approve, Deny
    Wait    bool     // question only: the lead can't go on with that step without the answer
}
// The result is text: "Raised req-N for the CEO." and what to do meanwhile.
```

- Effect: Workflow creates a [request](#requests) (`source: agent`, `request.created`), its title the request's summary, its body the request's body, unless the same one is open: same agent, kind, title and body (case and spacing aside) raised in the last 10 minutes, whose id it returns (`The CEO has it already, as req-N: it's still open.`).
- `approval`, or `question` with `Wait`: the call returns at once, telling the lead to wait for the answer before that step and work on anything else meanwhile (the CEO's decision, 2026-10-15: no blocking call, [Team tools → Waiting on approval](./team-tools.md#waiting-on-approval)). `question` without `Wait`, and `finding`: the call returns at once, telling it to keep working.
- **The answer** (`POST /api/requests/{id}/resolve`, an option or the CEO's own text, at most 8,000 characters) resolves the request and comes back as a [message](#messages) from the CEO, kind `answer`: `Message msg-N from the CEO (answer to req-N): On your approval "<summary>": <answer> Read it with read_message("msg-N"), then go on with what you asked it for.` A finding is answered the same way, with the lead's options: one team works on a project at a time, so its lead is who acts on it (the CEO, 2026-11-04). Answered once: a second answer is refused (`NOT_OPEN`). The message is kept before the request resolves: one that can't be is an error and the request stays open. When the deployment stops first, its open requests **expire** (`status: expired`), with nothing delivered. The CEO can also **dismiss** it (`POST /api/requests/{id}/dismiss`): the lead gets an `answer` message saying so, without an answer.
- Errors: `bad_kind`, `empty` (no title), `too_long` (a title over 120 characters or over one line, whose refusal says to put the rest in `Body`; a body over 8,000; over 6 options, or one over 80), `bad_option` (an empty, multi-line or repeated option). A member is never offered it ([Team tools](./team-tools.md#tools)).

### `request_merge`

```go
type RequestMergeArgs   struct { Branch string; Title string; Body string; Entry string; Into string; Land string } // branch: lead/<project>/<own team>/<topic>; title: what it brings, one line of at most 120 characters (required); body: what the CEO should know to review it, at most 8,000 (optional); entry: a wiki path, e.g. "backlog/LED-44.md"; into: a branch of the CEO's, e.g. "release/2.0"; land: a new branch it lands on, e.g. "f/export-csv", under the project's landing_prefixes (optional)
type RequestMergeResult struct { Review string; MergeRequests []string; Refreshed bool; Checks string } // the branch's review; one merge request per repo with new commits; refreshed: open ones on this branch were updated
```

- Effect: Workflow checks `Branch` against the policy (only the caller's own `lead/<project>/<team>/*`) and that `Entry` exists in the project's wiki, runs the checks, then creates (or refreshes) a [merge request](#work-items-and-merge-requests) for each repo in which merging the branch into its target would change something, all in the branch's open [review](../models/review.md) (or a new one), and raises its **review** request unless one is open. The review takes `Title` and `Body`, the lead's own name for the work and what to know about it, apart from the card (the CEO, 2026-11-04: the title is the agent's to set, explicitly); a refresh replaces both, so the CEO reads what the lead says now (a body left out leaves none). The target is `Into`, any branch of the CEO's: never an agent's; without it, a refreshed merge request keeps its own and a new one goes to its repo's default branch ([Where a merge goes](../features/pipeline-and-merges.md#where-a-merge-goes)). With it, every merge request the review has open moves to it too, even one whose repo has nothing new: a review never splits between branches unasked. One moved to a branch that holds all of its branch already is merged there and then, as when Foreman notices a merge outside it (the CEO, 2026-11-02: the review never shows it open for a merge that changes nothing), and the text says `api (mr-1) into release/2.0, which holds all of it already: counted as merged`; when nothing is left open it ends `: nothing left for the CEO.` Every repo is checked before anything is made, so a refusal makes nothing. Nothing is pushed anywhere: the CEO reviews it, then merges, requests changes or closes it ([Pipeline and merges](../features/pipeline-and-merges.md#review-and-merge)). The tool's text says `Review rev-1, api (mr-1) into main, web (mr-2) into trunk: waiting for the CEO.`, with `, refreshed` after each one it refreshed.
- Delivered to the lead when done: `From Foreman: merge requested for lead/ledger/atlas/settlement-v2 in ledger-api.`, or `From Foreman: checks fail on lead/ledger/atlas/settlement-v2: <first line>. No merge was requested.`
- Markers: `Mara · request_merge · lead/ledger/atlas/settlement-v2`, then the outcome.
- Errors (returned at once): `not_lead` (never seen in practice: the tool isn't offered to members), `branch_not_allowed`, `empty` (no title), `too_long` (a title over 120 characters or over one line, whose refusal says to put the rest in `Body`; a body over 8,000), `branch_not_found`, `agent_branch` (`Into` is an agent's branch, `wt/*` or `lead/*`), `landing_not_allowed` (`Land` isn't a valid name under the project's `landing_prefixes`, or the project has none), `landing_taken` (`Land` is a branch already, at another commit than `Into`, in a repo: the text names each), `nothing_to_merge` (merging it would change nothing in the branch it goes into, in any repo), `no_such_branch` (a repo with new commits, or one the review has open, lacks the branch it would merge into: the text names each, `web (no release/1.0)`, and says to ask the CEO to make it there, or split the work), `entry_not_found`, `submodule_changed` (as for `merge`), `uncommitted_changes` (commit first).
- Phasing (2026-10-17): built without the checks (`git-checks`), with its review request; kept as a team action for the project's Activity; the result names the merge requests, and the CEO decides on the review's page, in the project's Inbox. `Into` since 2026-11-02, any branch of the CEO's since 2026-11-06; `Land` since 2026-11-11 ([Where a merge goes](../features/pipeline-and-merges.md#where-a-merge-goes)): only proposed, the CEO makes it on Merge. The result says `api (mr-1) into main, landing on f/export-csv`.

### `fetch`

- Effect (the lead only; the CEO, 2026-11-04): Git fetches every remote of each code repo, in the CEO's repo (the worktrees share its refs), on the host and with the CEO's own credentials (their credential helper, keychain or SSH agent): only the CEO writes a repo's `.git/config`, so an agent can't point a remote elsewhere. `git fetch --all --prune --no-tags --no-recurse-submodules --no-write-fetch-head`: it moves only the remote-tracking branches (`origin/*`), pruning those deleted upstream; never a local branch (the CEO's included), a tag or a submodule (their URLs are the agents', fetched only by `set_submodule`, without credentials). It never waits on a prompt (no terminal, `GIT_TERMINAL_PROMPT=0`, no askpass), and a remote has 2 minutes.
- The text, a line per repo: `api: origin/main 1a2b3c4..5d6e7f8, origin/feat/y new, origin/feat/x gone upstream`, `web: up to date` or `web: no remote`; when something moved, how to merge it in (`merge("origin/main")`).
- A repo that fails leaves the others fetched; the call then fails (`fetch_failed`) with every repo's line, the failed one with git's message. Kept as a [team action](#team-actions).

### `prune`

```go
type PruneArgs struct { Branches []string } // optional: each the caller's own, or for the lead a teammate's wt/<project>/<agent>/<topic>
```

- Effect ([Sandboxes and git policy → Cleaning up branches](./sandboxes-and-git.md#cleaning-up-branches); the CEO, 2026-11-11): in each code repo, on the host, Workflow deletes the caller's merged and superseded branches (`wt/<project>/<self>/*`, or the lead's `lead/<project>/<team>/*`) and, for the lead, each teammate's merged `wt/<project>/<agent>/*`, and its superseded ones held by another branch of that same teammate; a branch of the caller's own named in `Branches` goes even unmerged. Each branch's standing is read as it goes (of two at one commit, one stays). It skips home branches (named, one is kept: `a home branch stays`) and keeps, saying why, one checked out (`checked out by tove`), one with a review open (`its review is open`) or that an open review merges into, the caller's own unmerged ones not named (`not merged: name it to delete it anyway`) and a teammate's held by no branch of its own (`not merged: only its owner deletes unmerged work`). Each head is kept under `refs/foreman/cleaned/<time>/<branch>` for 30 days.
- The text, a line per repo: `api: deleted wt/ledger/tove/led-1, lead/ledger/atlas/x; kept wt/ledger/tove/led-2 (checked out by tove)`, or `web: nothing to delete`; a named branch no repo has, `wt/ledger/tove/x: no such branch`; and `Foreman keeps a copy of each for 30 days.` when one went. Kept as a [team action](#team-actions).
- Errors: `not_yours` (a named branch that isn't the caller's, nor for the lead a teammate's: nothing is deleted), `git_error`.

### `withdraw_merge`

```go
type WithdrawMergeArgs struct { Branch, Reason, SupersededBy string } // branch, superseded_by: lead/<project>/<own team>/<topic>, each with a review open
```

- Effect: Workflow ends the caller's open [review](../models/review.md) of `Branch` as **withdrawn** ([Reviews → Withdrawn](../features/reviews.md#withdrawn)): every merge request still open in it becomes `withdrawn` (its note, the reason), the review keeps the reason and `superseded_by` (the review of `SupersededBy`), the CEO's draft comments on it are deleted, and its review request resolves (`withdrawn`). The work items its merge requests carried are freed: their commits are still on the lead branch, so they go to the lead's open merge requests whose branch holds them, in the same repo, or else wait for the next `request_merge` of a branch that does. The branch stays. Publishes `review.withdrawn`. The tool's text says `Withdrew review rev-1 of lead/ledger/atlas/small (api, web): it left the CEO's queue. The branch stays.`, with ` The CEO sees it superseded by rev-2 (lead/ledger/atlas/big).` when given, then where its hand-offs went.
- Kept as a [team action](#team-actions), refusals included.
- Errors: `not_lead` (never seen in practice), `branch_not_allowed` (either branch not the caller's own lead branch), `no_reason`, `too_long` (the reason, over the message limit), `superseded_by_itself`, `not_open` (no review open on `Branch`), `superseding_not_open` (`SupersededBy` has none: request it first), `not_superseded` (`SupersededBy` doesn't contain `Branch` in a repo the review has open: merge it in first, or leave it out).
- **Foreman withdraws too** (the CEO, 2026-10-22): when the CEO merges a repo of a review (Merge, or a merge Foreman notices outside it), every other open review of the same lead (deployment) with a merge request on that repo, whose every open merge request its target now holds (merging it would change nothing), is withdrawn the same way, with the reason `<branch>, merged, holds all of it` and superseded by the merged review; its work items go to the merged review's merge request in their repo. So the CEO can merge either one: merging the bigger withdraws the smaller, merging the smaller leaves the bigger open (it holds more). Nothing happens at `request_merge` time.

### `reviews`

- Effect: the caller's [reviews](../models/review.md) on its project (its deployment's: not another team's), open or decided in the last 7 days, newest first, as text: per review `rev-2 lead/ledger/atlas/big: <title> (<entry>)`; then each repo with its target and state, `api → main: changes requested · web → trunk: conflicts` (`waiting for the CEO`, `changes requested`, `conflicts`, `merged`, `dismissed (the CEO's note: …)`, `withdrawn`); `Withdrawn: <reason>, superseded by rev-N.` when it was; and while changes are requested, `The CEO's note: …` and the comments sent with the last Request changes, each as the lead's message shows it (on its line now, ` (changed since)` once outdated), line comments first. Built from what the review page reads (`workflow.Review`), so the two agree. `You have no review open, and none decided in the last 7 days.` when there is none.
- Read-only: not a [team action](#team-actions).
- Errors: `not_lead` (never seen in practice).

### `team`

- Effect: where each of the lead's teammates stands (the CEO, 2026-10-23), in roster order, one line each: `Tove, Engineer: running, last report 3m ago.`, then its worktrees, `api on wt/ledger/tove/led-44, uncommitted changes · web on wt/ledger/tove/led-44, merge under way`. The status is the [instance](../models/instance.md#statuses)'s, with `(<stop reason>)` when Foreman stopped it; the time is when its harness last reported (the activity line's, the one the Dashboard shows), not for an asleep or stopped one, and none until it reports after a Foreman restart (kept in memory). A teammate added to the team since the deploy shows `not in this deployment (joined the team since it was deployed).`; a lead alone gets `You have no teammates.`
- Only what Foreman itself knows crosses: never the activity line's text nor anything from the teammate's terminal ([Security](./security.md)). Worktrees are read on the host, so it wakes nobody.
- Read-only: not a [team action](#team-actions).
- Errors: `not_lead` (never seen in practice).

### Deliveries Foreman originates

| When | To | Delivered text |
|---|---|---|
| A merge request is merged (Merge, or outside Foreman) | its lead | `From Foreman: lead/ledger/atlas/integration was merged into main in ledger-api, as one commit. lead/ledger/atlas/integration now starts from main.`, naming its target; when the branch couldn't move to the target as it is now ([Merge to a target](../features/pipeline-and-merges.md#merge-to-a-target)), the second sentence is `lead/ledger/atlas/integration has commits since, or changes in the way, so it still holds what was merged: merge main into it with merge("main") before you go on.` |
| The CEO merges a review that holds all of another open review of the same lead | that lead | `From Foreman: the CEO merged lead/ledger/atlas/big, which holds all of lead/ledger/atlas/small, so Foreman withdrew the review of lead/ledger/atlas/small (rev-1, api, web). Don't request it again.` ([`withdraw_merge`](#withdraw_merge)); sent before the "target moved" notices, which then skip it |
| Another merge request into the same target of the same repo is merged | the lead of every other open one into it | `From Foreman: main moved in ledger-api (lead/ledger/atlas/integration was merged). Merge it into lead/ledger/atlas/settlement-v2 with merge("main"), resolve any conflicts and commit(), then call request_merge again.` ([When a target moves](../features/pipeline-and-merges.md#when-a-target-moves)) |
| Merge finds a conflict | that merge request's lead | `From Foreman: lead/ledger/atlas/settlement-v2 conflicts with main in ledger-api, so the CEO couldn't merge it. Merge main in with merge("main"), resolve the conflicts and commit(), then call request_merge again.` |
| Request changes on its review | the review's lead | `From the CEO: changes requested on lead/ledger/atlas/settlement-v2 (backlog/LED-44.md).`, the note, one line per comment, then `Fix them, commit(), then call request_merge again on the same branch.` ([Reviews → What the lead gets](../features/reviews.md#what-the-lead-gets)) |
| An agent's session starts fresh (a deploy, a wake; not a restart that continues its conversation) with earlier work in its worktrees | that agent | `From Foreman: your worktrees hold earlier work. Branches with work not merged yet: wt/ledger/tove/LED-44 (api). Changes not committed in web (on wt/ledger/tove/LED-44). Resume what is still wanted; new work goes on a new topic, made from the default branch with switch(<topic>), or from another branch with switch(<topic>, from). Changes you don't want: discard().`; for the lead also `Your reviews still open: lead/ledger/atlas/x (waiting for the CEO)` or `(changes requested: fix them, then request_merge again)`. Each part only when it applies; nothing when there is nothing |
| Dismiss on its review, its team kept working | the review's lead | `From Foreman: the CEO dismissed the review of lead/ledger/atlas/settlement-v2 without merging (ledger-api). Stop work on it, and don't request it again.`, then `The CEO's note: <note>` when there is one ([Reviews → What the lead gets](../features/reviews.md#what-the-lead-gets)). A team stopped with the close, or not running, is told nothing |
| "Owning team fixes now" on a finding | the named team's lead | `From the CEO: <team> fixes "<finding>" now.` |

## Messages

Messaging's unit: one `ask`, `reply`, `handoff` or `contribute` notice between two instances of one deployment, or a delivery Foreman originates. Stored as a [team message](../models/team-message.md) (agent-to-agent kinds) for the project's Activity.

```go
type Message struct {
    ID          string    // "msg-41": per deployment, for every message; read_message reads it, reply answers an ask by it
    Deployment  string
    From        string    // instance id, or "foreman" / "ceo" for originated deliveries
    To          string    // instance id
    Kind        string    // "ask" | "reply" | "handoff" | "contribute" | "notice"
    Body        string    // for a reply: only the output
    InReplyTo   string    // reply: the ask's msg id
    Artifact    string    // a branch or wiki path, optional
    CreatedAt   time.Time
    DeliveredAt time.Time // zero while queued
    State       string    // see the lifecycle
    Reason      string    // why it failed or expired
}
```

Lifecycle:

```
             recipient idle: Harness.Deliver
  queued ──────────────────────────────────▶ delivered
    │
    ├── deployment stopped ───────────────▶ expired   (reason: deployment_stopped; the sender is told if it still runs)
    └── Deliver fails 3 times ────────────▶ failed    (reason: the adapter's error; a blocked request on the recipient)
```

- **Ordering:** per recipient, FIFO by creation. When the recipient becomes idle, every queued message for it is delivered **in one input**, in order, so a burst of messages costs one turn and never interleaves with a turn in progress. One message is its notice; several are one line naming each: `3 new messages: msg-41 from Mara (ask), msg-42 from Rhea (reply to msg-40), msg-43 from Tove (handoff of wt/ledger/tove/LED-44). Read each with read_message.`
- **Recipient state:** `idle` → delivered now · `running` or `awaiting` → queued until the next `idle` · `queued` or `starting` → queued; delivered after the first `idle` · `stopped` within an active deployment → queued, and the sender gets the marker "Tove is stopped; msg-41 will be delivered when it restarts" (the stop already raised a blocked request) · deployment stopped → expired.
- **Scope:** `From` and `To` are always instances of `Deployment` (or Foreman/CEO). Names in `ask`, `reply` and `handoff` resolve only within the caller's deployment.

## Team actions

What the git tools did, for the CEO to follow (the CEO's decision, 2026-10-16): each call of `open`, `switch`, `commit`, `merge`, `abort_merge`, `discard`, `set_submodule`, `request_merge` or `withdraw_merge` is kept in `team_actions`, refusals included (a hand-off and a contribution are messages already). Workflow records it after the call and publishes `team.action`.

```go
type TeamAction struct {
    ID           int64
    DeploymentID string
    Instance     string        // who called it
    Tool         string        // switch, commit, merge, abort_merge, discard, set_submodule, request_merge, withdraw_merge
    Branch       string        // switch: to; merge: from; set_submodule: the branch committed on
    Repos        []ActionRepo  // what happened in each repo
    Error        string        // the refusal the agent got; "" when it went through
    CreatedAt    time.Time
}
type ActionRepo struct { Repo, Branch, Message, Commit string; Created bool; Conflicts []string } // Message: commit's, for this repo; set_submodule's, which says what it did
```

Shown in the project's [Activity](../features/project-activity.md#the-timeline), between the threads.

## Requests

Workflow's unit to the CEO. Stored as a [request](../models/request.md); kinds, actions and display rules are in [Requests](../features/requests.md).

```go
type Request struct {
    ID          string
    Kind        string   // "approval" | "question" | "review" | "finding" | "blocked"
    Source      string   // "agent" | "harness" (its waiting hook) | "foreman"
    Project     string
    Deployment  string
    Instance    string   // who raised it, or who it is about (blocked)
    Summary     string   // its title: one line
    Body        string   // the rest, when it needs more; "" for none
    Options     []string // first = primary
    Review      string   // review only
    Status      string   // "open" | "resolved" | "expired"
    Answer      string   // an option, or the CEO's own text
    CreatedAt, ResolvedAt time.Time
}
```

Lifecycle:

```
  open ──── CEO answers (an escalation) ────────────▶ resolved
    │ ──── its review has nothing open (review) ──────▶ resolved
    │ ──── the agent is active again (blocked) ───────▶ resolved
    │ ──── its session works again or ends (harness) ─▶ resolved
    │ ──── deployment stopped (an escalation) ─────────▶ expired
    └──── CEO dismisses it (any but a review) ────────▶ dismissed
```

How the answer travels back:

| Kind | On `Resolve` |
|---|---|
| `approval`, `question`, `finding` (an escalation) | A message from the CEO to the lead, kind `answer` ([`escalate`](#escalate)) |
| `review` | No answer: the decisions are `ReviewAction`s on its review. Request changes resolves it (the drafts go to the lead in one delivery); so does the review having nothing left open |
| `blocked` | An action on the instance through the Instance manager: restart now, restart later, switch profile (later) |
| `source: harness` (a question) | Not answered here: the CEO answers in the session, and its next activity resolves it ([Waiting for the CEO](#waiting-for-the-ceo)) |

**Dedup:** a new request with the same instance, kind and normalised summary and body as an `open` one created within 10 minutes isn't created; the open one is returned (its id is in the tool's result). Details: [Request](../models/request.md#invariants).

## Work items and merge requests

Stored as [work items](../models/work-item.md) and [merge requests](../models/merge-request.md); the flow is in [Pipeline and merges](../features/pipeline-and-merges.md).

Work item:

```
  handoff,       delivered       head found in a         request_merge  its merge request
  checks pass    to the lead     lead/<project>/<team>/* branch    includes it    is merged
 ────────────▶ 1 handed ────▶ 2 reviewing ────────────▶ 3 integrated ────▶ requested ──▶ merged
                  ▲                                                         │
                  │  handoff again, checks pass (from 0 or 2)               │ rejected
                  │                                                         ▼
                  └───────── 0 in worktree ◀───────────────────────────────┘
                                          ▲
                                          └── a handoff whose checks fail (note "checks fail")
```

**A change request moves nothing** (the CEO's decision, 2026-10-16). When the lead asks the author for changes, that is a plain `ask`, and the item stays where it is; the author's next `handoff` of the same branch moves it back to "handed". Rows move only on hand-offs, integration and merge request events, so the list never depends on an agent naming the right branch, and `ask` keeps no extra field.

| Transition | Trigger | Detected by |
|---|---|---|
| → 1 handed | `handoff` whose checks pass | Workflow |
| (stays 0, note "checks fail") | `handoff` whose checks fail | Workflow |
| 1 → 2 reviewing | the handoff delivery's `DeliveredAt` | Messaging event |
| 2 → 1 handed | the author hands the same branch off again (after a change request, a plain `ask`) | Workflow |
| 2 → 3 integrated | the item's branch head is an ancestor of one of the lead's `lead/<project>/<team>/*` heads | Git, checked right after the lead's `merge` or `commit` (which finishes a conflicted merge), the only ways a lead branch takes in work, and on `request_merge`; in every repo that has the branch |
| 3 → requested | `request_merge` on a branch containing it | Workflow |
| requested → merged / → 0 | its merge request merged / rejected | Workflow |
| requested → 3 integrated, or another merge request | its merge request withdrawn: to the superseding merged one in its repo, else to an open one of the lead's whose branch holds it, else free | Workflow |

Merge request:

```
          request_merge (checks pass)           Merge (on its review), or merged outside Foreman
  (none) ─────────────────────────────▶ open ─────────────────────────────────────────▶ merged
                                         │ ▲
               request_merge again       │ │  (refreshed: checks rerun; same request)
                                         │ └──┘
                                         ├── Dismiss (its review) ▶ rejected
                                         └── withdraw_merge · Foreman, when a merged review holds all of its review ▶ withdrawn
```

- `merged`, `rejected` and `withdrawn` are final. Entering `withdrawn` publishes `review.withdrawn`; Foreman's own withdrawal delivers to the lead. Entering `merged` or `rejected` delivers to the lead, and resolves its review's request once the review has nothing left open. Entering `merged` sends the "main moved" notice to the leads of every other open merge request on the same repo.
- Several merge requests can be open at once: one per (repo, lead branch).

## Events

Every state change is published on the in-process event bus; the web server streams them to the UI ([API → Events](./api.md#events)).

| Event | Payload | Emitted by | Used by |
|---|---|---|---|
| `instance.status` | instance id, status, task | Instance manager | Session's agent tabs, Dashboard, Deployment and Team rows |
| `instance.doing` | instance id, its activity line and when ([Harness adapters → Activity lines](./harness-adapters.md#activity-lines)) | Instance manager, from the harness's hooks | Deployment rows |
| `instance.marker` | instance id, marker text, kind (`ask`, `reply`, `handoff`, `contribute`, `escalate`, `merge`, `note`) | Messaging, Workflow | The project's Activity (not shown yet) |
| `team.message` | the [message](#messages), on creation and on each state change | Messaging | Activity |
| `team.action` | an [action](#team-actions), once recorded | Workflow | Activity |
| `request.created` / `request.resolved` | the [request](#requests) | Workflow | Dashboard, Inbox, sidebar badges, notifications |
| `pipeline.changed` | the work item or merge request | Workflow | Inbox and Deployment (In flight, Merged) |
| `review.withdrawn` | the review, its deployment and lead, `by` (`lead` or `foreman`), how many of the CEO's draft comments went with it | Workflow | Inbox, the toast warning of lost drafts |
| `deployment.created` / `deployment.stopped` | the deployment | Instance manager | Sidebar, Dashboard, Team page, Session, Deployment |
| `wiki.changed`, `config.changed`, `projects.changed` | as in [API](./api.md#events) | Projects & config, Wiki | Wiki, Board, Settings |

## Invariants

- **Identity comes from the token.** `Instance` and `Deployment` in every `ToolCall` and `HookEvent` are set by the agent link from the caller's token; nothing in a payload can claim another identity.
- **Messages never cross deployments.** Team-tool names resolve within the caller's deployment only.
- **Only `Output` and `Artifact` cross sandboxes** in a reply; a handoff names a branch, never a path.
- **Only the lead requests merges** (and withdraws them), only from its own `lead/<project>/<team>/*` branches, and **only the CEO's action merges**.
- **Only Foreman writes git.** Agents read it; every commit, merge and branch, in the code repos and the HQ alike, is a team tool run on the host ([Sandboxes and git policy → Agents and git](./sandboxes-and-git.md#agents-and-git)).
- **Nothing is typed into a session except deliveries**, and only when the recipient is idle (the CEO's own keystrokes excepted).
- **Every request has exactly one way back**: a tool result, a delivery or an instance action (see [Requests](#requests)).
- **Every state change is an event**, so the UI never polls for it.

