---
type: spec
status: planned
tags: [architecture, team-tools, requests, pipeline]
---

# Protocol

The messaging and workflow schema: who sends what, through which [building block](./building-blocks.md), as which struct, with which lifecycle, and what the UI hears. The models stay the single source of the **stored** fields ([Team message](../models/team-message.md), [Request](../models/request.md), [Work item](../models/work-item.md), [Pull request](../models/pull-request.md), [Instance](../models/instance.md)); this entry owns the wire and in-memory types and the interactions between blocks. Go structs are sketches: field names and comments are the contract, exact types are settled in code.

## Actors and channels

Three actors, and every interaction enters through exactly one channel:

| Actor | Channel | Struct | Received by |
|---|---|---|---|
| Agent | A team-tool call: harness → `foreman-agent mcp` → agent port | [`ToolCall`](#envelopes) | Agent link → **Messaging** (`ask`, `reply`) or **Workflow** (`handoff`, `escalate`, `open_pr`) |
| Agent | A harness hook: `foreman-agent hook <event>` → agent port | [`HookEvent`](#envelopes) | Agent link → **Instance manager** (status), which tells Messaging and Workflow through events |
| CEO | Keystrokes in a terminal pane | raw bytes | Web server → **Terminal** `Input` (no Foreman logic) |
| CEO | Line input (`POST /api/instances/:id/input`) | [`CEOInput`](#envelopes) | Web server → **Terminal** `Input`, as the same bytes typing would produce |
| CEO | Answering a request (Home, Changes) | `Answer` | Web server → **Workflow** `Resolve` |
| CEO | Send back, Discard (a pull request) | `PRAction` | Web server → **Workflow** |
| Foreman | Process exit, silence, rate limit, git or build error | internal | **Instance manager** / **Workflow** → blocked [request](#requests) |
| Foreman | Forge poll, project-repo watch | internal | **Workflow** → [pull request](#work-items-and-pull-requests) transitions |

Agents never reach each other except through Messaging, never reach the CEO except through Workflow, and never reach anything with a forge token.

## Envelopes

```go
// ToolCall: one team-tool call, already authenticated by the agent link.
type ToolCall struct {
    CallID     string          // the MCP call id; the result is matched to it
    Instance   string          // "mara@ledger": from the token, never from the payload
    Deployment string          // the instance's deployment: from the token too
    Tool       string          // "ask" | "reply" | "handoff" | "escalate" | "open_pr"
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

- A **delivery** is text the agent must read: a teammate's ask, a reply, a handoff to the lead, the CEO's answer, a rebase notice. Messaging hands it to `Instances.Deliver`, which waits until the recipient is `idle` and then calls the harness adapter's `Deliver` (type the text, then submit). It starts a turn.
- A **marker** is a note for the CEO's eyes only: "Mara asked Rhea (req-41)", "PR #128 opened". It is a UI event (`instance.marker`) shown **beside** the terminal: a thin strip on the session pane, and the Messages drawer. It is **never** written into the PTY, where it would corrupt the program's screen or be read as an instruction.

Every delivery is also shown as a marker. Delivered text is untrusted data: it is framed with its sender and capped ([Team tools → Delivery](./team-tools.md#delivery)).

## Team tools

Who may call each tool, how it blocks, which block handles it, and what each side sees. Role rules come from the [team definition](../models/team-definition.md); a tool not allowed to a role isn't listed in its MCP tool list at all.

| Tool | Who | Blocks the caller | Handled by |
|---|---|---|---|
| `ask` | anyone | no: returns the request id at once | Messaging |
| `reply` | the agent an `ask` was addressed to | no | Messaging |
| `handoff` | anyone | no: returns once the branch is fetched; checks run after | Workflow (+ Messaging for the delivery) |
| `escalate` | anyone | `approval`: yes, until answered · `question`: only with `wait: true` · `finding`: no | Workflow |
| `open_pr` | the lead only | no: returns once the branch is fetched; checks, push and PR follow | Workflow |

### `ask`

```go
type AskArgs   struct { Teammate string; Request string }  // teammate: agent id or display name, same deployment
type AskResult struct { RequestID string }                // "req-41"
```

- Effect: a [message](#messages) of kind `ask` from the caller to the teammate, `req-N` assigned, queued for delivery.
- Delivered to the teammate: `⇄ Mara asked (req-41): <request>` followed by `Reply with reply("req-41", output) when done.`
- Markers: `Mara → Rhea · ask · req-41`.
- Errors: `unknown_teammate`, `self` (asking oneself), `too_long` (over the delivery cap), `deployment_stopped`.

### `reply`

```go
type ReplyArgs   struct { RequestID string; Output string; Artifact string } // artifact: a branch or a wiki path, optional
type ReplyResult struct {}
```

- Effect: a message of kind `reply` to the agent that asked, `in_reply_to` the ask. **Only `Output` and `Artifact` cross**; nothing else of the replier's context does.
- Delivered to the requester: `⇄ Rhea replied to req-41: <output>` (+ `Artifact: <ref>`), then `Resume the task that asked for it.`
- Markers: `Rhea → Mara · reply · req-41`.
- Errors: `unknown_request` (no such `req-N` in the deployment), `not_addressee` (the ask wasn't to the caller), `already_replied` (one reply per ask: further information is a new `ask`), `bad_artifact`, `too_long`.

### `handoff`

```go
type HandoffArgs   struct { Teammate string; Branch string; Summary string } // branch: wt/<caller>/<card>
type HandoffResult struct { WorkItem string; Checks string }                  // checks: "running"
```

- Effect: Workflow asks Git to fetch `Branch` from the caller's clone into the mirror (policy: a member may hand off only `wt/<self>/*`), then runs the project checks in a throwaway container ([Checks](./sandboxes-and-git.md#checks)), creating or advancing the [work item](#work-items-and-pull-requests).
- Checks pass: the branch is fetched into the teammate's clone and delivered to the teammate: `⇄ Tove handed off wt/tove/LED-44: <summary>`. The caller gets `✓ Checks pass: wt/tove/LED-44 handed to Mara.` as a delivery when idle.
- Checks fail: nothing goes to the teammate; the caller gets `✗ Checks fail on wt/tove/LED-44: <first lines>. Fix and hand off again.`
- Markers: `Tove → Mara · handoff · wt/tove/LED-44`, then the checks outcome.
- Errors (returned at once): `unknown_teammate`, `branch_not_allowed` (not `wt/<self>/*`), `branch_not_found`, `nothing_new` (the branch has no commits beyond `main`).

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
- `approval`, or `question` with `Wait`: the tool call stays open until the CEO answers; the answer is its result, and the agent continues from the same point. The instance is `awaiting` meanwhile. Harnesses time tool calls out ([Team tools → Pausing on approval](./team-tools.md#pausing-on-approval)): shortly before the harness's limit the call returns `Pending: true`, and the answer later arrives as a delivery: `✓ CEO answered req <id>: <answer>. Resume where you paused.`
- `question` without `Wait`, and `finding`: the call returns at once; the answer arrives as a delivery: `✓ CEO answered: <answer>. Continuing.`
- Errors: `bad_kind`, `too_long`.

### `open_pr`

```go
type OpenPRArgs   struct { Branch string; Title string; Summary string } // branch: lead/<own team>/<topic>
type OpenPRResult struct { PullRequest string; Updated bool; Checks string } // updated: an open PR on this branch was refreshed
```

- Effect: Workflow fetches `Branch` from the lead's clone (a lead may rewrite its own `lead/<team>/*`), runs the checks, then pushes and opens (or updates) the [pull request](#work-items-and-pull-requests) and its **review** request.
- Delivered to the lead when done: `✓ PR #128 opened: <url>` (no forge: `✓ lead/atlas/settlement-v2 published for review in ledger-api.`), or `✗ Checks fail on lead/atlas/settlement-v2: <first lines>. No PR was opened.`
- Markers: `Mara · open_pr · lead/atlas/settlement-v2`, then the outcome.
- Errors (returned at once): `not_lead` (never seen in practice: the tool isn't offered to members), `branch_not_allowed`, `branch_not_found`, `nothing_to_merge` (no commits beyond `main`).

### Deliveries Foreman originates

| When | To | Delivered text |
|---|---|---|
| A pull request is merged | its lead | `✓ PR #128 merged into main.` |
| Another PR on the same repo is merged | every other open PR's lead | `⇄ main moved (PR #128 merged). Rebase lead/atlas/settlement-v2 onto main and push.` ([When main moves](../features/pipeline-and-prs.md#when-main-moves)) |
| The forge reports a conflict | that PR's lead | the same rebase notice |
| Review comments on the forge | the PR's lead | `⇄ Review on PR #128 by <user>: <comment>` |
| Send back, or closed on the forge | the PR's lead | `✗ PR #128 sent back: <notes>. Its work items are back in your hands.` |
| "Owning team fixes now" on a finding | the named team's lead | `ⓘ CEO: <team> fixes "<finding>" now.` |
| A soul or team edit | each running instance | `ⓘ Your definition changed: …` ([Session prompt → Updates](./session-prompt.md#updates)) |

## Messages

Messaging's unit: one `ask`, `reply` or `handoff` between two instances of one deployment, or a delivery Foreman originates. Stored as a [team message](../models/team-message.md) (agent-to-agent kinds) for the Messages drawer.

```go
type Message struct {
    ID          string    // "req-41": per deployment, for ask (a reply carries its ask's id)
    Deployment  string
    From        string    // instance id, or "foreman" / "ceo" for originated deliveries
    To          string    // instance id
    Kind        string    // "ask" | "reply" | "handoff" | "notice"
    Body        string    // for a reply: only the output
    InReplyTo   string    // reply: the ask's id
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

- **Turns per project (until the Git epic):** while the agents of a project share its mounted folder, they take turns. A delivery goes to an agent only when **no other agent in the same project** (in any deployment on it) is `running`; otherwise it waits, like a delivery to a busy recipient. This gate covers every Foreman-originated delivery (messages, answers, notices). The CEO's own typing bypasses it; the UI warns instead ([Sessions](../features/sessions.md#talking-to-the-team)). The [Git epic](../../backlog/epics/git.md) removes the gate: with a worktree per agent, agents work in parallel.
- **Ordering:** per recipient, FIFO by creation. When the recipient becomes idle, every queued message for it is delivered **in one input**, in order, so a burst of messages costs one turn and never interleaves with a turn in progress.
- **Recipient state:** `idle` → delivered now · `running` or `awaiting` → queued until the next `idle` · `queued` or `starting` → queued; delivered after the first `idle` · `stopped` within an active deployment → queued, and the sender gets the marker "Tove is stopped; req-41 will be delivered when it restarts" (the stop already raised a blocked request) · deployment stopped → expired.
- **Scope:** `From` and `To` are always instances of `Deployment` (or Foreman/CEO). Names in `ask`, `reply` and `handoff` resolve only within the caller's deployment.

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
    PullRequest string   // review only
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
    │ ──── its PR leaves open (review) ──────────────▶ resolved
    │ ──── the instance restarts fine (blocked) ─────▶ resolved
    └──── deployment stopped / instance gone ───────▶ expired
```

How the answer travels back:

| Kind | On `Resolve` |
|---|---|
| `approval`, `question` with `wait` | The open tool call (`CallID`) returns the answer; if it already returned `Pending`, a delivery carries it |
| `question`, `finding` | A delivery to the raiser (`✓ CEO answered: …`); a finding's action may also deliver to another team's lead |
| `review` | No answer: the CEO acts on the forge; Send back is a `PRAction`. The request resolves when its PR leaves `open` |
| `blocked` | An action on the instance through the Instance manager: restart now, restart later, switch harness |
| `source: harness` (approval or question) | Typed into the session with the harness adapter's `Deliver`; also resolved by the next `busy` hook if the CEO answered in the terminal |

**Dedup:** a new request with the same instance, kind and normalised summary as an `open` one created within 10 minutes isn't created; the open one is returned (its `RequestID` is the tool result). Details: [Request](../models/request.md#invariants).

## Work items and pull requests

Stored as [work items](../models/work-item.md) and [pull requests](../models/pull-request.md); the flow is in [Pipeline and pull requests](../features/pipeline-and-prs.md).

Work item:

```
  handoff,       delivered       head found in a         open_pr        PR
  checks pass    to the lead     lead/<team>/* branch    includes it    merged
 ────────────▶ 1 handed ────▶ 2 reviewing ────────────▶ 3 integrated ────▶ in PR ────▶ merged
                  ▲                │                                        │
                  │      lead asks │ the author (?)            PR closed or │ sent back
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
| 2 → 3 integrated | the item's branch head is an ancestor of one of the lead's `lead/<team>/*` heads | Git, checked when the lead goes `idle` and on `open_pr` |
| 3 → in PR | `open_pr` on a branch containing it | Workflow |
| in PR → merged / → 0 | its PR merged / closed or sent back | Workflow |

Pull request:

```
           open_pr (checks pass, pushed)              forge: merged, or main contains the head (no forge)
  (none) ─────────────────────────────▶ open ─────────────────────────────────────────▶ merged
                                         │ ▲
                     open_pr again       │ │  (updated: new head, checks rerun, same number and request)
                                         │ └──┘
                                         └── forge: closed · Send back · Discard (CoS) ───▶ closed
```

- `merged` and `closed` are final. Entering either resolves the review request and delivers to the lead. Entering `merged` sends the rebase notice to the leads of every other open PR on the same repo.
- Several PRs can be open at once: one per (repo, lead branch).

## Events

Every state change is published on the in-process event bus; the web server streams them to the UI ([API → Events](./api.md#events)).

| Event | Payload | Emitted by | Used by |
|---|---|---|---|
| `instance.status` | instance id, status, task | Instance manager | Sessions tabs, Home, Project and Team rows, avatars |
| `instance.line` | instance id, last output line (throttled) | Terminal | Project settings rows |
| `instance.marker` | instance id, marker text, kind (`ask`, `reply`, `handoff`, `escalate`, `pr`, `note`) | Messaging, Workflow | The marker strip on Sessions panes, the Messages drawer |
| `team.message` | the [message](#messages), on creation and on each state change | Messaging | Messages drawer |
| `request.created` / `request.resolved` | the [request](#requests) | Workflow | Home, Changes, sidebar badges, notifications |
| `pipeline.changed` | the work item or pull request | Workflow | Changes |
| `deployment.created` / `deployment.stopped` | the deployment | Instance manager | Sidebar, Team page, Sessions |
| `wiki.changed`, `config.changed`, `workspace.changed` | as in [API](./api.md#events) | Workspace & config, Wiki | Overview, Board, Settings |

## Invariants

- **Identity comes from the token.** `Instance` and `Deployment` in every `ToolCall` and `HookEvent` are set by the agent link from the caller's token; nothing in a payload can claim another identity.
- **Messages never cross deployments.** Team-tool names resolve within the caller's deployment only.
- **Only `Output` and `Artifact` cross sandboxes** in a reply; a handoff moves a branch, through Foreman, never a path.
- **Only the lead opens pull requests**, only from its own `lead/<team>/*` branches.
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
