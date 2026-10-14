---
type: spec
status: planned
tags: [architecture, runtime, sessions, acp]
---

# Building blocks

The pieces Foreman is made of, what each one is responsible for, the interface it offers, and how sure we are it works. This entry is the map: every other architecture entry details one block. Interfaces are sketched in Go (TypeScript for the UI) for precision; exact signatures are settled in code, but a change to a block's *responsibility* or *contract* changes this entry first.

Every arrow crosses an interface ([Modularity](./overview.md#modularity)): any block can be replaced without its users noticing.

## The big picture

Three places run code: the CEO's **browser**, the **`foreman` binary** on the CEO's machine, and one **container per agent**.

```
┌──────────────────────────────── Browser ──────────────────────────────────┐
│ Screens: Home · Sessions · Project · Team · Agent · Settings · Chief of Staff│
│                 │ never talk to the network themselves                       │
│ ForemanClient ──┼── real client: HTTP + event stream + terminal WebSockets   │
│ (per area,      └── demo client: seed data + simulated events, "demo data"   │
│  in main.tsx)        badge (until that area's backend exists)              │
└───────────────────────────────────┬────────────────────────────────────────┘
                                    │ 127.0.0.1:8888, access token
┌───────────────────────── foreman (one Go binary) ──────────────────────────┐
│ EDGE        Web server · Access guard                                       │
│ CORE        Workflow · Messaging · Instance manager · Agent link            │
│             Harness adapters · Terminal · Execution engine + Image builder  │
│             Git (worktrees per agent)                                       │
│ FOUNDATION  Workspace & config · Wiki · State store · Event bus · Secrets · │
│             Forge adapter                                                   │
└──────┬────────────────────────────────────────────────────▲────────────────┘
       │ start · attach · resize · build (Podman REST API)    │ agent port 127.0.0.1:<p>,
       ▼                                                      │ one token per instance
┌────────────────────── Agent containers (Podman) ─────────────┴──────────────┐
│ one per instance:  harness TUI ── stdio MCP + hooks ──▶ foreman-agent        │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Who calls whom

An arrow means "calls through an interface". Blocks below the dashed line are used by everything above and are left out of the arrows.

```
                               Web server (+ Access guard)
          ┌───────────────┬──────────────┬───────────────┬─────────────────┐
          │ attach, input │ CEO messages │ answers,      │ projects, teams,│
          ▼               ▼              ▼ merge actions ▼ settings, wiki  │
   ┌──────────────┐  ┌───────────┐  ┌──────────┐  ┌────────────────────┐  │
   │   Instance   │◀─│ Messaging │◀─│ Workflow │─▶│ Workspace & config │  │
   │   manager    │  └───────────┘  └──┬────┬──┘  │ Wiki               │  │
   └─┬───┬───┬──┬─┘        ▲           │    │     └────────────────────┘  │
     │   │   │  │          │           ▼    ▼                              │
     │   │   │  │          │         Git   Forge adapter                   │
     │   │   │  │ register │ ask, reply,          ▲ escalate, handoff,     │
     │   │   │  │ tokens   │ handoff              │ request_merge, …       │
     │   │   │  └────────▶ Agent link ────────────┘                        │
     │   │   │              ▲    │ hook events ──▶ Instance manager        │
     │   │   │              │                                              │
     │   │   │        foreman-agent (in each container)                    │
     │   │   ▼                                                             │
     │   │  Harness adapter                                                │
     │   ▼                                                                 │
     │  Terminal ──▶ Execution engine ──▶ Podman container | direct process│
     ▼                                                                     │
   Image builder                                       events ─────────────┘
 ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─
 Used by all of the above: State store · Event bus (→ the UI's live updates) · Secrets
```

Every arrow, in words:

| From | To | Why |
|---|---|---|
| Web server | Instance manager | Open a terminal view, type into it, deploy, stop |
| Web server | Messaging | The CEO's line-mode messages; the Messages drawer |
| Web server | Workflow | Answer a request; Merge to main, Create PR or Reject a merge request; Sync a reserved repo |
| Web server | Workspace & config, Wiki | Projects, teams, settings, wiki pages and boards; edits to souls, checkboxes, card moves |
| Web server | Event bus | Stream live events to the browser |
| Workflow | Messaging | Deliver answers, notices ("PR #128 opened", "main moved") to agents |
| Workflow | Git | Switch, commit and update an agent's worktrees for its tool calls; check a hand-off branch, detect integration; on the CEO's actions, merge to main, push a branch for a PR, or Sync a reserved repo (Git epic) |
| Workflow | Forge adapter | On the CEO's Create PR: open, then poll, comment on and close pull requests |
| Workflow | Workspace & config, Wiki | Gates and checks from `project.toml`; `contribute` notes into the project's inbox; team files changed by the Chief of Staff's tools |
| Messaging | Instance manager | Deliver a message when the recipient is idle; start a stopped recipient |
| Agent link | Messaging | `ask`, `reply`, `handoff` tool calls |
| Agent link | Workflow | `switch`, `commit`, `update`, `escalate`, `handoff`, `contribute`, `request_merge`, and the Chief of Staff's tools |
| Agent link | Instance manager | Hook events (busy, idle, waiting for the CEO) |
| Instance manager | Git | The instance's worktrees, before it starts |
| Instance manager | Image builder | The project's image |
| Instance manager | Harness adapter | Launch spec and files; delivering text; reading hook events; diagnosing exits |
| Instance manager | Agent link | Register an instance's token when it starts, revoke it when it stops |
| Instance manager | Terminal | Create one per running process; input, resize |
| Terminal | Execution engine | The process: bytes in and out, resize, stop, wait |

## Inside an agent container

```
┌──────────────────── container for tove@ledger (Podman, rootless) ────────────────────┐
│                                                                                     │
│  harness TUI (Claude Code / Pi / Hermes)         reads ──▶ CLAUDE.md / AGENTS.md     │
│     │  stdio MCP: ask · reply · handoff · contribute · escalate · request_merge     │
│     │             switch · commit · update (git writes, run by Foreman on the host) │
│     │  hooks: "foreman-agent hook stop | notification | …"                          │
│     ▼                                                                               │
│  foreman-agent ──────────── tcp: host.containers.internal:<agent port>, token ──────┼──▶ Agent link
│                                                                                     │
│  Mounts                                                                             │
│   /…/ledger/.sandboxes/tove/ledger-api   Tove's worktree (wt/tove/*), rw            │
│   /…/ledger/ledger-api/.git              the repo every worktree shares, ro         │
│   /…/ledger/.sandboxes/tove/project      the live project repo (project.toml, wiki):│
│                                          rw for the lead, else ro                   │
│                                          (its .git ro for every agent)              │
│   $CLAUDE_CONFIG_DIR     the instance's private config: prompt, MCP + hooks config,  │
│                          history, memory (rw)                                        │
│   …/credentials/<h>/     the harness's shared login file (rw)                        │
│   ~/.gitconfig           ro, if it exists                                            │
│  Never mounted: the rest of ~, ~/.ssh, ~/.config, .foreman/ (secrets, state, teams), │
│  the CEO's working files, other agents' worktrees                                   │
└─────────────────────────────────────────────────────────────────────────────────────┘
```

## The core blocks

### 1. Execution engine (`runner`)

Starts one process attached to a terminal and gives back its byte stream. It knows nothing about agents, harnesses or Foreman.

```go
type Runner interface {
    Start(ctx, Spec) (Process, error)
    List(ctx) ([]Process, error)       // still-running processes it started, found by label (re-attach after a restart)
}
type Process interface {
    io.ReadWriter                      // terminal output out, input in: raw bytes
    Resize(Size) error
    Stop(grace time.Duration) error
    Wait() (exitCode int, err error)
    Labels() map[string]string
}
type Spec struct { Image; Command; Dir; Env; Mounts; Size; Labels }
```

Implementations, named by **where the process runs** (both give it a terminal):
- **Podman** (`runner/podman`): a rootless container. **Podman** allocates the terminal inside the container (`podman run -t`); Foreman reads and writes it through the REST API's attach stream and resizes it with an API call. Survives a Foreman restart. The production engine.
- **Direct** (`runner/direct`): a process on the host, in a pseudo-terminal that **Foreman** allocates. **No isolation.** For tests and development, and selectable by an explicit setting (`engine`) with a "no isolation" warning if ever needed for real use.

#### 1b. Image builder (`runner/<engine>`)

Each engine comes with the image builder that matches it. The Instance manager asks it for the project's image and puts the answer in `Spec.Image`.

**Phasing:** phase 1 builds one image for every project (from `pclaude`'s Dockerfile, plus `foreman-agent`); the four-layer chain comes later ([Agent images](./tech-stack.md#agent-images)).

```go
type Images interface {
    Ensure(ctx, project) (ref string, err error) // builds or tags the chain as needed: base → harness → workspace → project
    Refresh(ctx, layer) error                    // rebuild without cache (new harness versions), then everything below
    Prune(ctx) (reclaimed int64, err error)      // Foreman's outdated images only
    Status(ctx) []ImageStatus                    // Settings → Maintenance
}
```

- **Podman**: the four-layer chain ([Agent images](./tech-stack.md#agent-images)).
- **Direct**: a no-op. `Ensure` returns no image, and the Direct engine ignores `Spec.Image`: the process uses the host's tools.

Details: [Session runner](./session-runner.md).

### 2. Terminal (`terminal`)

One live process and its buffer. It owns the process handle, keeps the server-side screen (for snapshots) and the raw log, fans the output out to every viewer, and takes input. It does not decide whether a process should run, or restart it.

```go
type Terminal interface {
    Attach() (View, error)             // a viewer: snapshot, then live output
    Input(p []byte) error              // keystrokes, or a message being delivered
    Resize(Size) error
    Done() <-chan struct{}             // closed when the process exits
    ExitCode() (int, bool)
}
type View interface {
    Snapshot() []byte                  // recreates the current screen on a fresh terminal
    Output() <-chan []byte
    Close()
}
```

Built from a `Process`, a `Screen` (server-side terminal emulator, [Terminal state](./terminal-state.md)) and a `Log` ([Persistence](./persistence.md#session-logs)).

### 3. Harness adapter (`harness`)

Everything harness-specific (Claude Code, Pi, Hermes, a custom command, the fake harness in tests), behind one interface.

```go
type Harness interface {
    // What to run: image, command, env, and the files written into the
    // instance's private config dir: the session prompt (CLAUDE.md / AGENTS.md),
    // the MCP config pointing at foreman-agent, the hooks config.
    Launch(Instance) (runner.Spec, []File, error)
    // How to type a message into the running TUI (text, then Enter; bracketed paste…).
    Deliver(t Terminal, message string) error
    // What a hook event means: idle, busy, waiting for the CEO (a dialog, a question), done.
    Interpret(HookEvent) Status
    // Recognises a crash or rate limit from the exit, for a blocked request.
    Diagnose(exitCode int, tail []byte) Diagnosis
}
```

Details: [Harness adapters](./harness-adapters.md), [Session prompt](./session-prompt.md).

### 4. Agent link (`agentlink` + `foreman-agent`)

How an agent inside a container talks to Foreman. Structured only: Foreman never infers events by reading terminal output, except inside a harness adapter as a last resort for a harness without hooks.

- **`foreman-agent`**: a tiny static binary in every container (built with Foreman, carried by the harness image layer). The harness starts `foreman-agent mcp` as a local (stdio) MCP server, which every harness supports; hooks run `foreman-agent hook <event>`. It relays both to Foreman.
- **The agent port**: **one** port that Foreman opens on the host's loopback, separate from the UI's. Every container connects to it through Podman's host alias (`host.containers.internal`); none exposes a port. Each instance has its own token (container environment), which is its identity: an agent can't speak for another.
- `foreman-agent` reaches Foreman through one address (`tcp://…` today), so another transport (a mounted Unix socket on Linux, say) is a change in one place.

```go
// What the agent link hands to the rest of Foreman, already authenticated:
type ToolCall  struct { Instance; Tool string; Args json.RawMessage }   // → Messaging / Workflow
type HookEvent struct { Instance; Event string; Data json.RawMessage }  // → Instance manager
type Tokens interface { Register(instance) (token string); Revoke(instance) }  // used by the Instance manager
```

Details: [Team tools](./team-tools.md#transport). Schema: [Protocol → Envelopes](./protocol.md#envelopes).

### 5. Instance manager (`instances`)

Decides which instances run: one per member of each active [deployment](../models/deployment.md). It creates their terminals (Image builder → Harness → Agent link token → Execution engine → Terminal), keeps their status from hook events, restarts after crashes, queues beyond the concurrency limit, and re-attaches after a Foreman restart.

```go
type Instances interface {
    Ensure(deployment) error           // start what should run, stop what shouldn't
    Status(id) Status                  // starting · running · idle · awaiting · done · stopped · queued
    Terminal(id) (Terminal, error)
    Deliver(id, message) error         // hands the message to the harness's Deliver when the instance is idle; queues otherwise
    Watch() <-chan StatusChange
}
```

Details: [Agent runtime](./agent-runtime.md).

### 6. Messaging (`messaging`)

Moves messages between agents, and between agents and the CEO: `ask`, `reply`, `handoff` and `contribute` notices, CEO answers. It numbers them (`req-N` per deployment), stores them, queues them per recipient and delivers through `Instances.Deliver`. Every delivery also produces a **marker** for the UI (shown beside the terminal, never written into it). A message to an agent that isn't running asks the instance manager to start it. It knows nothing about git, PRs or approvals.

```go
type Messaging interface {
    Send(Message) (ID, error)          // from an agent's tool call, or from the CEO
    Log(deployment) []Message          // the Messages drawer
}
```

Details: [Team tools](./team-tools.md), [Team message](../models/team-message.md). Schema: [Protocol → Messages](./protocol.md#messages).

### 7. Workflow (`workflow`)

Gives events their meaning: `switch`, `commit` and `update` → git writes in the caller's worktrees, through Git; `escalate` and harnesses waiting for the CEO → a [request](../features/requests.md) on Home (and its answer back); `handoff` → a [work item](../models/work-item.md); `contribute` → a note in the project's `wiki/inbox/` and a notice to the lead; `request_merge` → checks and a [merge request](../models/merge-request.md) with its review; the CEO's Merge to main, Create PR or Reject → Git and the Forge adapter; project gates; crashes → blocked requests; the Chief of Staff's tools → team files and config changes, on a branch that comes back through `request_merge`.

```go
type Workflow interface {
    Handle(ToolCall) (Result, error)   // switch, commit, update, escalate, handoff, contribute, request_merge, Chief of Staff tools
    Resolve(request, answer) error     // the CEO answers a request
    Requests(filter) []Request
}
```

Details: [Requests](../features/requests.md), [Pipeline and merges](../features/pipeline-and-merges.md). Schema: [Protocol → Requests](./protocol.md#requests), [work items and merge requests](./protocol.md#work-items-and-merge-requests).

### 8. Git (`git`)

Everything Foreman does with repositories on the agents' behalf, on the host. Each instance gets a **git worktree** of each code repo, sharing the repo with the CEO's checkout and every other agent ([Sandboxes and git policy](./sandboxes-and-git.md)). Agents only read git: each repo's `.git` is mounted read-only, and every git write is a team tool that Foreman runs here, with the repo's hooks off and one git command at a time per repo ([Agents and git](./sandboxes-and-git.md#agents-and-git)). Agents never push anywhere.

```go
type Git interface {
    Prepare(instance, repo) (path string, err error) // the instance's worktree of the repo, made if missing
    Switch(instance, repo, branch) error             // switch its worktree to the branch, made from main if missing
    Commit(instance, repo, message) (commit string, err error) // everything changed in its worktree (or the live project repo, for its writers), on its branch, authored as the agent
    Update(instance, repo, branch) (conflicts []string, err error) // merge a branch into its worktree; conflicts stay as markers
    Prune(instance, repo) error                      // its deployment ended: delete its wt/* branches already merged into main
    Contains(repo, branch, into string) (bool, error) // integration detection: is the work in a lead branch?
    Merge(repo, branch) error                        // the CEO's Merge to main: merge the branch into main on the host
    Push(repo, branch, remote) error                 // on the CEO's Create PR (the branch) or Sync (a reserved repo's main), with Foreman's token
    Fetch(repo) error                                // after a merge on the forge, and as Sync's first step: fetch the forge's main into the repo
}
```

**Phasing.** `Prepare` (a worktree and home branch per agent) arrives in epic 1, [Terminal multiplexer](../../backlog/epics/terminal-multiplexer.md); `Switch`, `Commit` and `Update`, behind the `switch`, `commit` and `update` team tools, arrive with the agent link in epic 2, together with hand-offs that name a branch (nothing to fetch: every worktree shares the repo). `Prune`, `Contains`, `Merge`, `Push` and `Fetch` (merge plumbing) arrive with the [Git epic](../../backlog/epics/git.md).

## The foundation blocks

No open risk here: standard techniques. Each is still behind its own interface.

### Workspace & config (`workspace`)

The files the CEO owns: the workspace folder and [workspace repo](../models/workspace.md#the-workspace-repo) (team files, settings), each project's [project repo](../models/project-config.md#the-project-repo) (`project.toml`); and the file watcher that notices their changes. All TOML.

```go
type Workspace interface {
    Projects() []Project;  Project(id) (Project, error)
    Candidates() []Candidate                         // folders that aren't projects yet (Add a project)
    AddProject(folder) (Project, error)              // creates the project repo: project.toml and a starter wiki
    Teams() []Team;        Team(id) (Team, error)
    SaveTeam(Team, version) error                    // version-guarded; one commit per save
    Settings() Settings;   SaveSettings(Settings) error
    Changes() <-chan Change                          // file watcher: config, team files, wiki, Containerfiles
}
```

Details: [Workspace](../models/workspace.md), [Persistence → File watching](./persistence.md#file-watching).

### Wiki (`wiki`)

Reads project wikis through the agentic-wiki engine; renders entries for the reader; edits one line at a time. The lead writes the wiki as plain files in the live project repo, and commit them through `commit()`; Foreman only reads it, plus the CEO's checkbox ticks and card moves and the `contribute` notes it files in `inbox/`, each committed on its own (one commit per write).

```go
type Wiki interface {
    Tree(project) Tree
    Entry(project, path) (Rendered, error)           // HTML, headings, frontmatter, checkboxes with their lines
    Board(project, id) (Board, error)                // [[tool.wikiview.board]] from wiki.toml
    SetCheckbox(project, path, line, done, version) error
    SetField(project, path, field, value, version) error
}
```

Details: [Wiki engine](./wiki-engine.md).

### State store (`store`)

Runtime state in `state.db` (SQLite): deployments, instances, messages, requests, work items, merge requests, read markers, UI preferences. Each block owns its own tables and reaches them only through its own store interface (`store.Deployments`, `store.Messages`, …); no block reads another's tables.

Details: [Persistence](./persistence.md).

### Event bus (`events`)

In-process publish and subscribe: status changes, new and resolved requests, messages and markers, file changes. The Web server streams them to the browser (the UI's live updates and notifications).

```go
type Bus interface {
    Publish(Event)
    Subscribe(filter) (<-chan Event, cancel func())
}
```

Details: [API → Events](./api.md#events), [Protocol → Events](./protocol.md#events).

### Secrets (`secrets`)

Forge tokens and the access token, in a 0600 file; never mounted into containers. Harness logins are not here (they live in their own credentials folder).

```go
type Secrets interface { Set(key, value) error; Get(key) (string, error); Has(key) bool }
```

Details: [Secrets](../models/secrets.md).

### Forge adapter (`forge`)

```go
type Forge interface {
    OpenPR(repo, branch, title, body) (PR, error)
    PR(repo, number) (PR, error)                     // state, checks, mergeable; polled
    Comment(repo, number, text) error
    Close(repo, number) error
}
```

Implementations: GitHub, Gitea (also Forgejo), none. Used only on the CEO's Create PR or Sync; never merges. Details: [Forge adapters](./forge-adapters.md).

### Web server + UI (`server`, `access`, `ui/`)

- **Web server**: the API, the event stream, the terminal WebSockets, behind the **Access guard** (token, Host and Origin checks). It holds no logic of its own: it calls the blocks above. Details: [API](./api.md), [Security](./security.md).
- **UI**: screens use only the `ForemanClient` interface. `ui/src/main.tsx`, the UI's composition root, chooses per area between the **real client** (HTTP, events, WebSockets) and the **demo client** (in-memory seed data from the prototype, simulated events, a visible "demo data" badge). As each area's backend lands, its line switches to the real client ([Full UI on a demo client](../../backlog/epics/full-ui-demo.md)).

```ts
interface ForemanClient {
  workspace(): Promise<Workspace>
  instances(): Promise<Instance[]>
  openTerminal(instance: string, handlers: PtyHandlers): PtyConnection
  // … one method per API call; the types are generated from the Go structs
}
```

### Chief of Staff

Not a separate block: an instance like any other (Instance manager, Harness, Terminal, Execution engine) whose team tools include extra ones (`present_choices`, `show_panel`, `create_team`, `hire`, …), handled by Workflow. The drawer in the UI renders its tool calls as a chat. Details: [Chief of Staff](../features/chief-of-staff.md).

## How the blocks work together

The flows everything else is made of. Time runs downwards. The [journey](../journey.md) walks through all of them in one story.

**1. Starting an instance** (deploying a team, a restart, a hire)

```
Instance manager      Image builder     Harness adapter        Agent link     Execution engine     Terminal
      │ Git.Prepare(instance, repo) for each code repo → its worktrees (made if missing)
      │ Ensure(project) ──▶│
      │◀── image ref ──────│
      │ Launch(instance) ───────────────────▶│ spec + files (prompt, MCP config → foreman-agent, hooks)
      │◀──────────────────────────────────────│
      │ Register(instance) → token ──────────────────────────────▶│
      │ Start(spec + token) ─────────────────────────────────────────────────────▶│ container up
      │ New(process, screen, log) ──────────────────────────────────────────────────────────────▶│
      │ status: starting → (first hook "idle") → idle
```

**2. The CEO watches and types** (no Foreman logic in between: raw bytes)

```
Browser ──WS──▶ Web server ──Attach()──▶ Terminal ──▶ View: snapshot, then live output ──▶ Browser
Browser ──keys──▶ Web server ──Input()──▶ Terminal ──▶ Process (the harness's TUI)
```

**3. An agent asks a teammate** (the run loop, epic 2)

```
Mara's harness   foreman-agent   Agent link   Messaging                       Instance manager        Tove's harness
 ask(tove,"…") ──▶ MCP ─────────▶ ToolCall ──▶ store req-41; marker ─▶ Event bus ─▶ UI: "Mara asked Tove (req-41)"
 ◀── "req-41" ──────────────────────────────── (non-blocking: Mara keeps working)
                                              Deliver(tove, text) ──▶ waits: Tove idle
                                                                      Harness.Deliver ──────────▶ "⇄ Mara asked (req-41): …"
                                                                                                    … works …
 reply(req-41, out) ◀────────── (the same path, from Tove; delivered to Mara when she is idle) ─────┘
```

**4. An agent needs the CEO** (approval; epic 4)

```
Agent ── escalate(approval, …) ──▶ Agent link ──▶ Workflow: create request ──▶ Event bus ──▶ Home + notification
      (the tool call stays open: the agent is paused)
CEO answers in the UI ──▶ Workflow.Resolve ──▶ the open tool call returns the answer ──▶ the agent resumes
```

**5. Hand-off, integration and a merge** (hand-off from epic 2; work items, integration and merges from the Git and Workflow epics)

```
Member ── switch(branch), then commit(message) ──▶ Workflow: Git.Switch, Git.Commit in its worktrees
Member ── handoff(lead, branch, summary) ──▶ Workflow: Git.Commit (pending work), policy check, work item (one repo: nothing to fetch)
                                          └▶ Messaging: deliver "⇄ Tove handed off …" to the lead
Lead ── update(branch) ──▶ Workflow: Git.Update into lead/<team>/<topic> in its worktree ──▶ Git.Contains → work item integrated
Lead ── request_merge(branch, entry) ──▶ Workflow: checks ──▶ merge request + request "Review" ──▶ Home
CEO: Merge to main ──▶ Workflow ──▶ Git.Merge  │  Create PR ──▶ Git.Push ──▶ Forge.OpenPR  │  Reject ──▶ reason delivered to the lead
(after Create PR) CEO merges on the forge ──▶ Forge.PR (polling) ──▶ Workflow: merged, lead told, other leads told to merge main in ──▶ Git.Fetch
```

**6. A screen whose backend isn't built yet** (Full UI on a demo client)

```
Screen ──▶ ForemanClient ──▶ demo client: seed data, simulated events ──▶ the screen shows a "demo data" badge
                          ╲─▶ real client (once that area's backend lands; one line in main.tsx)
```

## What is proven, and what isn't

| | Block | Status |
|---|---|---|
| | Terminal streaming to the browser, access guard | **Built and tested** (Direct engine) |
| | Terminal snapshots on reload | Built; one rare edge case open (a pending line wrap), parked as polish |
| **A** | Execution engine on Podman: start, attach, resize, re-attach after a Foreman restart, from Go | **Proven on macOS** (2 ms keystrokes; [results](../../backlog/prove-a-podman-engine.md)); Linux not yet run |
| **B** | Harness + engine: one login shared by many concurrent containers | **Proven for Claude Code on macOS**, print mode: the login file alone, bind-mounted into each private config dir, survives 12 sessions refreshing at once ([results](../../backlog/prove-b-shared-login.md)); mid-session refresh in interactive sessions is checked in D; Linux not yet run |
| **C** | Agent link: a container reaching the host's loopback port on macOS and Linux | **Proven on macOS** ([results](../../backlog/prove-c-agent-port.md)); Linux (pasta) not yet run |
| **D** | Harness (Claude Code): `Deliver` into a busy TUI, and `Interpret` from hooks (idle, waiting for the CEO) | **Unproven, core to messaging** |
| **E** | Harness: prompt file + stdio MCP config accepted by Claude Code, Pi, Hermes | **Proven for Claude Code on macOS** ([results](../../backlog/prove-e-prompt-and-mcp-config.md)); Pi and Hermes when set up |

A–E are proven before the blocks that depend on them are built: one small experiment each, run on the CEO's macOS and Fedora machines through a `just` recipe, with a yes/no outcome written back into the specs ([Prove epic](../../backlog/epics/prove.md)).

## Code today, against these blocks

- `internal/runner` + `runner/direct`: the Execution engine and its Direct implementation.
- `internal/terminal`: the Terminal block (one process, its log, its views). Its snapshot is the log's tail until `internal/screen` is wired in ([Terminal fidelity](../../backlog/terminal-fidelity.md)).
- `internal/store`: the State store (`state.db`, embedded migrations); `Deployments` and `Instances` so far, opened at startup.
- `cmd/foreman/sessions.go`: a start-by-instance map, scaffolding for `--demo-session` until the Instance manager exists ([technical debt](../../technical-debt.md)).
- `internal/screen` (parked, see [Terminal fidelity](../../backlog/terminal-fidelity.md)), `internal/sessionlog`: the Terminal's `Screen` and `Log`.
- `internal/server`, `internal/access`, `ui/` (with `ForemanClient` and its real implementation): the Web server and UI.
