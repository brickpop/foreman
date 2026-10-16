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
| Agent | A team-tool call: harness → `foreman-agent mcp` → agent port | [`ToolCall`](#envelopes) | Agent link → **Messaging** (`ask`, `reply`) or **Workflow** (`switch`, `commit`, `merge`, `abort_merge`, `handoff`, `contribute`, `escalate`, `request_merge`) |
| Agent | A harness hook: `foreman-agent hook <event>` → agent port | [`HookEvent`](#envelopes) | Agent link → **Instance manager** (status), which tells Messaging and Workflow through events |
| CEO | Keystrokes in a terminal pane | raw bytes | Web server → **Terminal** `Input` (no Foreman logic) |
| CEO | Line input (`POST /api/instances/:id/input`) | [`CEOInput`](#envelopes) | Web server → **Terminal** `Input`, as the same bytes typing would produce |
| CEO | Answering a request (Home, Changes) | `Answer` | Web server → **Workflow** `Resolve` |
| CEO | Merge to main, Create PR on the forge, Reject (a merge request) | `MergeAction` | Web server → **Workflow** |
| Foreman | Process exit, silence, rate limit, git or build error | internal | **Instance manager** / **Workflow** → blocked [request](#requests) |
| Foreman | Forge poll, project-repo watch | internal | **Workflow** → [merge request](#work-items-and-merge-requests) transitions |

Agents never reach each other except through Messaging, never reach the CEO except through Workflow, and never reach anything with a forge token.

## Envelopes

```go
// ToolCall: one team-tool call, already authenticated by the agent link.
type ToolCall struct {
    CallID     string          // the MCP call id; the result is matched to it
    Instance   string          // "mara@ledger": from the token, never from the payload
    Deployment string          // the instance's deployment: from the token too
    Tool       string          // "ask" | "reply" | "switch" | "commit" | "merge" | "abort_merge" | "handoff" | "contribute" | "escalate" | "request_merge"
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
| `waiting` | It waits **for the user**: a permission dialog, a multiple-choice question, or a question it asked in plain text before stopping | `Notification` | `awaiting`, with a request on Home ([Waiting for the CEO](#waiting-for-the-ceo)) |
| `ended` | The harness is exiting | `SessionEnd` | (the Terminal's exit sets `stopped`) |

`awaiting` means a request of the instance blocks it: an open `escalate(approval)`, or a `waiting` hook. `idle` and `waiting` must never be confused: an instance merely idle after finishing raises nothing ([Harness adapters → Idle and waiting detection](./harness-adapters.md#idle-and-waiting-detection)).

### Waiting for the CEO

Whenever the harness reports it is waiting for the user, Workflow raises a request on Home, as if the agent had escalated:

- **Kind:** `approval` when the harness exposes choices (a permission dialog, a multiple-choice question), with those choices as `Options`; otherwise `question`. `Source: harness` (the panel reads "waiting in its session").
- **Content:** the question text and the choices, when the harness's hook payload carries them; otherwise "<Name> is waiting for your input" and an `Open session` action.
- **Answer:** the CEO's choice or text is typed into the session through the harness adapter's `Deliver` (the instance is waiting, so it is not queued). Answering in the terminal directly works too: the next `busy` hook resolves the request.
- The instance is `awaiting` while the request is open.

## Text into a session: deliveries and markers

Anything Foreman writes into a session's terminal is **typed input**: harnesses are full-screen programs that read the terminal as their keyboard. So there are two different things, never mixed:

- A **delivery** is text the agent must read: a teammate's ask, a reply, a handoff to the lead, the CEO's answer, a "main moved" notice. Messaging hands it to `Instances.Deliver`, which waits until the recipient is `idle` and then calls the harness adapter's `Deliver` (type the text, then submit). It starts a turn. **A delivery is always one line of plain words, with no symbols** (the CEO's decisions, 2026-10-15): a multi-line paste reaches Claude Code wrapped as pasted content, which it treats as possibly not the user's ([experiment D](../../backlog/prove-d-claude-delivery-hooks.md)). A message's notice names it and quotes its start; the agent reads the whole message with `read_message`. Foreman's own notices are short sentences that fit the line.
- A **marker** is a note for the CEO's eyes only: "Mara asked Rhea (msg-41)", "Merge requested: lead/atlas/integration". It is a UI event (`instance.marker`) shown **beside** the terminal, in the Team traffic panel ([Sessions](../features/sessions.md#team-traffic)). It is **never** written into the PTY, where it would corrupt the program's screen or be read as an instruction.

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
| `handoff` | members (the lead has no `wt/` branch to hand off) | no: returns once the branch is checked; checks run after | Workflow (+ Messaging for the delivery) |
| `contribute` | anyone | no: returns once the note is written | Workflow (+ Messaging for the delivery) |
| `escalate` | anyone | `approval`: yes, until answered · `question`: only with `wait: true` · `finding`: no | Workflow |
| `request_merge` | the lead only | no: returns once the branch and entry are checked; checks and the review request follow | Workflow |

### `ask`

```go
type AskArgs   struct { Teammate string; Request string }  // teammate: agent id or display name, same deployment
type AskResult struct { RequestID string }                // "msg-41"
```

- Effect: a [message](#messages) of kind `ask` from the caller to the teammate, `msg-N` assigned, queued for delivery.
- Delivered to the teammate: `Message msg-41 from Mara (ask): <the request's start>… Read it with read_message("msg-41"); answer with reply("msg-41", output).`
- Markers: `Mara → Rhea · ask · msg-41`.
- Errors: `unknown_teammate`, `self` (asking oneself), `too_long` (over the delivery cap), `deployment_stopped`.

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

### `switch`

```go
type SwitchArgs   struct { Branch string }                                // "wt/tove/LED-44"; the lead: "lead/atlas/settlement-v2"
type SwitchResult struct { Created bool; Repos []string }               // created: the branch was new (made from main); the repos switched
```

- Effect: Workflow asks Git to switch the caller's worktrees, in each of its code repos, to `Branch`, creating it from `main` where it doesn't exist yet. `Branch` must be one of the caller's own: `wt/<agent>/<topic>` for a member, `lead/<team>/<topic>` for the lead (the lead never works on `wt/` branches). The same call starts new work and goes back to earlier work (e.g. a branch whose merge request was rejected). A lead can keep several lead branches, and so several merge requests, at once. Agents can't create branches themselves: each repo's `.git` is read-only ([Sandboxes and git policy → Agents and git](./sandboxes-and-git.md#agents-and-git)).
- Nothing is delivered; no marker.
- Errors: `uncommitted_changes` (commit first, so no work is carried from one branch to another), `branch_not_allowed` (not one of the caller's own), `bad_branch` (not a valid branch name), `git_error`.

### `commit`

```go
type CommitArgs   struct { Message string }
type CommitResult struct { Commits []RepoCommit }        // one per repo that had changes
type RepoCommit   struct { Repo string; Branch string; Commit string }
```

- Effect: Workflow asks Git to stage everything in each of the caller's worktrees that has changes and commit it on the branch checked out there, authored as the agent (from its token). For the lead it also commits its changes in the live project repo, on its `main` (other agents can't write that checkout). Agents can't commit themselves: every repo's `.git`, the project repo's included, is mounted read-only ([Sandboxes and git policy → Agents and git](./sandboxes-and-git.md#agents-and-git)). After an `merge` with conflicts, the commit concludes the merge.
- Nothing is delivered; no marker.
- Errors: `nothing_to_commit`, `conflicts_left` (conflict markers still in a file of an unfinished merge), `git_error` (with git's message).

### `merge`

```go
type MergeArgs   struct { Branch string }               // e.g. "lead/atlas/integration", "main", "wt/tove/LED-44"
type MergeResult struct { Repos []RepoMerge }
type RepoMerge   struct { Repo string; Merged bool; Conflicts []string } // files left with conflict markers
```

- Effect: Workflow asks Git to merge `Branch` into the branch checked out in each of the caller's worktrees whose repo has it. Any branch of the repo may be merged in (reading is open to everyone); only the caller's own worktrees change. On a conflict the markers stay in the files: the agent resolves them and calls `commit`, which finishes the merge (it refuses while markers are left), or backs out with `abort_merge`. Agents never rebase: branches are shared from their first commit (a teammate may already have merged one), and integration detection and the forge's PRs follow commits, which a rebase would replace (decided 2026-10-16).
- Nothing is delivered; a merge into a lead branch may let Workflow detect an integration ([Work items and merge requests](#work-items-and-merge-requests)).
- Errors: `branch_not_found` (in none of the caller's repos), `uncommitted_changes` (commit first, so a merge never mixes with unsaved work), `git_error`.
- `merge` was called `update` until 2026-10-16: the git word, which agents already know.

### `abort_merge`

```go
type AbortMergeArgs   struct {}
type AbortMergeResult struct { Repos []string } // where a merge was backed out
```

- Effect: Workflow asks Git to back out of the unfinished merge in each of the caller's worktrees that has one (`git merge --abort`): the branch and its files are as they were before the merge. A repo whose merge finished cleanly keeps it.
- Nothing is delivered; no marker.
- Errors: `no_merge` (no merge under way in the caller's repos), `git_error`.

### `handoff`

```go
type HandoffArgs   struct { Teammate string; Branch string; Summary string } // branch: wt/<caller>/<topic>
type HandoffResult struct { WorkItem string; Checks string }                  // checks: "running"
```

- Effect: a hand-off is an ask that Foreman understands: it names a committed branch, so Foreman can track it. It never commits: committing has one way, `commit` (decided 2026-10-16), and a caller with uncommitted changes is refused, so what is handed off is exactly what was committed. Workflow checks `Branch` against the policy (a member may hand off only `wt/<self>/*`; [Sandboxes and git policy](./sandboxes-and-git.md#branch-policy)). Every worktree shares the repo, so nothing is fetched: the lead sees the branch at once. Workflow then runs the project checks in a throwaway container ([Checks](./sandboxes-and-git.md#checks)), creating or advancing the [work item](#work-items-and-merge-requests).
- Checks pass: the hand-off is delivered to the teammate: `Message msg-43 from Tove (handoff of wt/tove/LED-44): <the summary's start>… Read it with read_message("msg-43").` The caller gets `From Foreman: checks pass, wt/tove/LED-44 was handed to Mara.` as a delivery when idle.
- Checks fail: nothing goes to the teammate; the caller gets `From Foreman: checks fail on wt/tove/LED-44: <first line>. Fix it and hand off again.`
- Markers: `Tove → Mara · handoff · wt/tove/LED-44`, then the checks outcome.
- Errors (returned at once): `unknown_teammate`, `branch_not_allowed` (not `wt/<self>/*`), `uncommitted_changes` (commit first), `branch_not_found`, `nothing_new` (the branch has no commits beyond `main`).
- Phasing: in [Messaging and hand-offs](../../backlog/epics/messaging-handoffs.md) a hand-off is the policy check and the delivery only; checks and work items arrive with the [Git](../../backlog/epics/git.md) epic.

### `contribute`

```go
type ContributeArgs   struct { Kind string; Title string; Body string } // kind: "finding" | "proposal" | "note"
type ContributeResult struct { Path string }                            // "project/wiki/inbox/2026-10-14-tove-flaky-settle-test.md"
```

- Effect: Workflow writes a new entry `project/wiki/inbox/<yyyy-mm-dd>-<agent>-<slug>.md` into the live project repo, the slug made from `Title`. Its frontmatter: `type: note`, `kind`, `author` (the caller's agent id, from its token, never from the arguments) and `deployment`; its body is `# <Title>`, then `Body`. A title is one line (at most 120 characters), so nothing in it can reach the frontmatter; a taken file name gets `-2`, `-3`. The commit holds only that file, authored as the agent, so the lead's unfinished edits in the same checkout stay out of it. Agents without write access to the knowledge base contribute only this way: there is no writable folder for them ([Sandboxes and git policy](./sandboxes-and-git.md#project-knowledge-base)).
- Delivered to the caller's lead: `Message msg-44 from Tove (contribution): <title>, in project/wiki/inbox/<file>.` The lead decides what becomes of it.
- Markers: `Tove → Mara · contribute · msg-44`, like every other kind; the Team traffic panel shows the title.
- Errors: `bad_kind`, `too_long`.

### `escalate`

```go
type EscalateArgs struct {
    Kind    string   // "approval" | "question" | "finding"
    Summary string
    Options []string // optional; the first is the primary action
    Wait    bool     // question only: block like an approval
}
type EscalateResult struct {
    RequestID string // the request on Home
    Answer    string // set when the call blocked until the CEO answered
    Pending   bool   // the call stopped blocking before an answer (see below)
}
```

- Effect: Workflow creates a [request](#requests) (`source: agent`), deduplicated against open ones. Markers: `⚠ Mara escalated (approval): <summary>`.
- `approval`, or `question` with `Wait`: the call returns at once, telling the agent to wait for the answer before that step (the CEO's decision, 2026-10-15: no blocking call, [Team tools → Waiting on approval](./team-tools.md#waiting-on-approval)). The instance is `awaiting` until the CEO answers; the answer arrives as a delivery: `From the CEO, on <request>: <answer>. Resume where you paused.`
- `question` without `Wait`, and `finding`: the call returns at once; the answer arrives as a delivery: `From the CEO, on <request>: <answer>.`
- Errors: `bad_kind`, `too_long`.

### `request_merge`

```go
type RequestMergeArgs   struct { Branch string; Entry string } // branch: lead/<own team>/<topic>; entry: a wiki path, e.g. "backlog/LED-44.md"; required for a lead, optional for the Chief of Staff (which describes its change in the request)
type RequestMergeResult struct { MergeRequests []string; Refreshed bool; Checks string } // one per repo with new commits; refreshed: open ones on this branch were updated
```

- Effect: Workflow checks `Branch` against the policy (only the caller's own `lead/<team>/*`; the Chief of Staff: its own `foreman/config-N`) and that `Entry` exists in the project's wiki (a lead must name one; the Chief of Staff may leave it empty and describe the change in the request instead), runs the checks, then creates (or refreshes) a [merge request](#work-items-and-merge-requests) for each repo in which the branch has commits beyond `main`, each with its **review** request. Nothing is pushed anywhere: the CEO chooses Merge to main, Create PR on the forge, or Reject ([Pipeline and merges](../features/pipeline-and-merges.md#review-and-merge)).
- Delivered to the lead when done: `From Foreman: merge requested for lead/atlas/settlement-v2 in ledger-api.`, or `From Foreman: checks fail on lead/atlas/settlement-v2: <first line>. No merge was requested.`
- Markers: `Mara · request_merge · lead/atlas/settlement-v2`, then the outcome.
- Errors (returned at once): `not_lead` (never seen in practice: the tool isn't offered to members), `branch_not_allowed`, `branch_not_found`, `nothing_to_merge` (no commits beyond `main` in any repo), `entry_not_found`.

### Deliveries Foreman originates

| When | To | Delivered text |
|---|---|---|
| A merge request is merged (Merge to main, or on the forge) | its lead | `From Foreman: lead/atlas/integration was merged into main in ledger-api.` |
| Another merge request on the same repo is merged | the lead of every other open one | `From Foreman: main moved (lead/atlas/integration was merged). Merge it into lead/atlas/settlement-v2 with merge("main"), resolve any conflicts and commit(), then call request_merge again.` ([When main moves](../features/pipeline-and-merges.md#when-main-moves)) |
| Merge to main finds a conflict, or the forge reports one | that merge request's lead | the same notice |
| The CEO chooses Create PR | the merge request's lead | `From Foreman: the CEO opened PR #128 on the forge for lead/atlas/settlement-v2: <url>` |
| Review comments on the forge (after Create PR) | the merge request's lead | `From Foreman: a review comment on PR #128 by <user>: <its start>… Read it with read_message("msg-N").` (stored as a message, kind `notice`) |
| Reject, or its PR closed on the forge | the merge request's lead | `From the CEO: the merge of lead/atlas/settlement-v2 was rejected: <reason>. Its work items are back in your hands; switch back to the branch to rework it.` |
| "Owning team fixes now" on a finding | the named team's lead | `From the CEO: <team> fixes "<finding>" now.` |
| A soul or team edit | each running instance | `From Foreman: your definition changed. Read it with read_message("msg-N").` (the changed sections, stored as a message of kind `notice`; [Session prompt → Updates](./session-prompt.md#updates)) |

## Messages

Messaging's unit: one `ask`, `reply`, `handoff` or `contribute` notice between two instances of one deployment, or a delivery Foreman originates. Stored as a [team message](../models/team-message.md) (agent-to-agent kinds) for the Team traffic panel.

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

- **Ordering:** per recipient, FIFO by creation. When the recipient becomes idle, every queued message for it is delivered **in one input**, in order, so a burst of messages costs one turn and never interleaves with a turn in progress. One message is its notice; several are one line naming each: `3 new messages: msg-41 from Mara (ask), msg-42 from Rhea (reply to msg-40), msg-43 from Tove (handoff of wt/tove/LED-44). Read each with read_message.`
- **Recipient state:** `idle` → delivered now · `running` or `awaiting` → queued until the next `idle` · `queued` or `starting` → queued; delivered after the first `idle` · `stopped` within an active deployment → queued, and the sender gets the marker "Tove is stopped; msg-41 will be delivered when it restarts" (the stop already raised a blocked request) · deployment stopped → expired.
- **Scope:** `From` and `To` are always instances of `Deployment` (or Foreman/CEO). Names in `ask`, `reply` and `handoff` resolve only within the caller's deployment.

## Team actions

What the git tools did, for the CEO to follow (the CEO's decision, 2026-10-16): each call of `switch`, `commit`, `merge` or `abort_merge` is kept in `team_actions`, refusals included (a hand-off and a contribution are messages already). Workflow records it after the call and publishes `team.action`.

```go
type TeamAction struct {
    ID           int64
    DeploymentID string
    Instance     string        // who called it
    Tool         string        // switch, commit, merge, abort_merge
    Branch       string        // switch: to; merge: from
    Message      string        // commit
    Repos        []ActionRepo  // what happened in each repo
    Error        string        // the refusal the agent got; "" when it went through
    CreatedAt    time.Time
}
type ActionRepo struct { Repo, Branch, Commit string; Created bool; Conflicts []string }
```

Shown in the [Team traffic](../features/sessions.md#team-traffic) panel, between the threads.

## Requests

Workflow's unit to the CEO. Stored as a [request](../models/request.md); kinds, actions and display rules are in [Requests](../features/requests.md).

```go
type Request struct {
    ID          string
    Kind        string   // "approval" | "question" | "review" | "finding" | "blocked"
    Source      string   // "agent" | "harness" (its waiting hook) | "foreman" | "cos"
    Project     string
    Deployment  string   // absent for cos
    Instance    string   // who raised it, or who it is about (blocked); absent for cos
    Summary     string
    Options     []string // first = primary
    MergeRequest string  // review only
    CallID      string   // approval (or question with wait): the open tool call to answer
    Status      string   // "open" | "resolved" | "expired"
    Answer      string
    CreatedAt, ResolvedAt time.Time
}

type Answer struct { Request string; Choice string; Text string } // an option, or free text (approval, question)
```

Lifecycle:

```
  open ──── CEO answers (Resolve) ─────────────────▶ resolved
    │ ──── its merge request leaves open (review) ────▶ resolved
    │ ──── the instance restarts fine (blocked) ─────▶ resolved
    └──── deployment stopped / instance gone ───────▶ expired
```

How the answer travels back:

| Kind | On `Resolve` |
|---|---|
| `approval`, `question` with `wait` | The open tool call (`CallID`) returns the answer; if it already returned `Pending`, a delivery carries it |
| `question`, `finding` | A delivery to the raiser (`From the CEO, on <request>: …`); a finding's action may also deliver to another team's lead |
| `review` | No answer: Merge to main, Create PR and Reject are `MergeAction`s. The request resolves when its merge request leaves `open` |
| `blocked` | An action on the instance through the Instance manager: restart now, restart later, switch harness |
| `source: harness` (approval or question) | Typed into the session with the harness adapter's `Deliver`; also resolved by the next `busy` hook if the CEO answered in the terminal |

**Dedup:** a new request with the same instance, kind and normalised summary as an `open` one created within 10 minutes isn't created; the open one is returned (its `RequestID` is the tool result). Details: [Request](../models/request.md#invariants).

## Work items and merge requests

Stored as [work items](../models/work-item.md) and [merge requests](../models/merge-request.md); the flow is in [Pipeline and merges](../features/pipeline-and-merges.md).

Work item:

```
  handoff,       delivered       head found in a         request_merge  its merge request
  checks pass    to the lead     lead/<team>/* branch    includes it    is merged
 ────────────▶ 1 handed ────▶ 2 reviewing ────────────▶ 3 integrated ────▶ requested ──▶ merged
                  ▲                │                                        │
                  │      lead asks │ the author (?)                         │ rejected
                  │                ▼                                        ▼
                  └───────── 0 in worktree ◀───────────────────────────────┘
          handoff again, checks pass      ▲
                                          └── a handoff whose checks fail (note "checks fail")
```

(?) Whether a change request from the lead moves a work item back is undecided: [open question 1](#open-questions).

| Transition | Trigger | Detected by |
|---|---|---|
| → 1 handed | `handoff` whose checks pass | Workflow |
| (stays 0, note "checks fail") | `handoff` whose checks fail | Workflow |
| 1 → 2 reviewing | the handoff delivery's `DeliveredAt` | Messaging event |
| 2 → 0 (undecided) | a change request from the lead to the author while the item is in review | [open question 1](#open-questions) |
| 2 → 3 integrated | the item's branch head is an ancestor of one of the lead's `lead/<team>/*` heads | Git, checked when the lead goes `idle` and on `request_merge` |
| 3 → requested | `request_merge` on a branch containing it | Workflow |
| requested → merged / → 0 | its merge request merged / rejected | Workflow |

Merge request:

```
          request_merge (checks pass)           Merge to main, or merged on the forge (after Create PR)
  (none) ─────────────────────────────▶ open ─────────────────────────────────────────▶ merged
                                         │ ▲
               request_merge again       │ │  (refreshed: checks rerun; pushed to its PR if any; same request)
                                         │ └──┘
                                         └── Reject · its PR closed on the forge ───────▶ rejected
```

- Create PR on the forge doesn't change the state: the merge request stays `open`, now with a PR to watch.
- `merged` and `rejected` are final. Entering either resolves the review request and delivers to the lead. Entering `merged` sends the "main moved" notice to the leads of every other open merge request on the same repo.
- Several merge requests can be open at once: one per (repo, lead branch).

## Events

Every state change is published on the in-process event bus; the web server streams them to the UI ([API → Events](./api.md#events)).

| Event | Payload | Emitted by | Used by |
|---|---|---|---|
| `instance.status` | instance id, status, task | Instance manager | Sessions tabs, Home, Project and Team rows, avatars |
| `instance.line` | instance id, last output line (throttled) | Terminal | Project settings rows |
| `instance.marker` | instance id, marker text, kind (`ask`, `reply`, `handoff`, `contribute`, `escalate`, `merge`, `note`) | Messaging, Workflow | The marker strip on Sessions panes, the Messages drawer |
| `team.message` | the [message](#messages), on creation and on each state change | Messaging | Team traffic panel |
| `team.action` | an [action](#team-actions), once recorded | Workflow | Team traffic panel |
| `request.created` / `request.resolved` | the [request](#requests) | Workflow | Home, Changes, sidebar badges, notifications |
| `pipeline.changed` | the work item or merge request | Workflow | Changes |
| `deployment.created` / `deployment.stopped` | the deployment | Instance manager | Sidebar, Team page, Sessions |
| `wiki.changed`, `config.changed`, `workspace.changed` | as in [API](./api.md#events) | Workspace & config, Wiki | Overview, Board, Settings |

## Invariants

- **Identity comes from the token.** `Instance` and `Deployment` in every `ToolCall` and `HookEvent` are set by the agent link from the caller's token; nothing in a payload can claim another identity.
- **Messages never cross deployments.** Team-tool names resolve within the caller's deployment only.
- **Only `Output` and `Artifact` cross sandboxes** in a reply; a handoff names a branch, never a path.
- **Only the lead requests merges**, only from its own `lead/<team>/*` branches (the Chief of Staff from its own config branches), and **only the CEO's action merges or reaches a forge**.
- **Only Foreman writes git.** Agents read it; every commit, merge and branch, in the code repos and the project repo alike, is a team tool run on the host ([Sandboxes and git policy → Agents and git](./sandboxes-and-git.md#agents-and-git)).
- **Nothing is typed into a session except deliveries**, and only when the recipient is idle (the CEO's own keystrokes excepted).
- **Every request has exactly one way back**: a tool result, a delivery, a forge event or an instance action (see [Requests](#requests)).
- **Every state change is an event**, so the UI never polls for it.

## Open questions

1. **Does a change request move a work item back?** The scenario: Changes → In flight shows each work item and its stage, e.g.
   - "Idempotency keys · Tove · `wt/tove/LED-44` · handed to lead"
   - "Rate limits · Tove · `wt/tove/LED-51` · handed to lead"

   The lead messages Tove: "fix the rounding in LED-44". Tove receives the message either way; that part isn't in question. The question is only what the **In flight** list shows afterwards:
   - **Option A: the LED-44 row goes back to "with the author"** automatically. Foreman can only know which row is meant if the lead names the branch when asking (an extra, optional `about: wt/tove/LED-44` on `ask`); without it, nothing moves.
   - **Option B: the rows stay as they are** until Tove hands LED-44 off again (a new `handoff` of the same branch moves that row back to "handed to lead"). Change requests are then plain messages, and the list only moves on handoffs, integration and PR events.

   Not decided.
