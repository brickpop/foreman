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
│ Screens: Dashboard · Sessions · Project · Team · Agent · Settings · Chief of Staff│
│                 │ never talk to the network themselves                       │
│ ForemanClient ──── real client: HTTP + event stream + terminal WebSockets    │
│ (main.tsx)          (an area not built yet refuses: "Not built yet")        │
└───────────────────────────────────┬────────────────────────────────────────┘
                                    │ 127.0.0.1:8888, access token
┌───────────────────────── foreman (one Go binary) ──────────────────────────┐
│ EDGE        Web server · Access guard                                       │
│ CORE        Workflow · Messaging · Instance manager · Agent link            │
│             Harness adapters · Terminal · Execution engine + Image builder  │
│             Git (worktrees per agent)                                       │
│ FOUNDATION  Home & config · Wiki · State store · Event bus · Secrets ·      │
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
   │   Instance   │◀─│ Messaging │◀─│ Workflow │─▶│ Home & config      │  │
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
| Web server | Messaging | The CEO's Line; the Activity panel; the countdown on the CEO's draft |
| Web server | Workflow | Answer a request; a review's comments and decisions (Request changes, Merge, Dismiss; later Create PR); Sync a reserved repo |
| Web server | Home & config, Wiki | Projects, teams, settings, wiki pages and boards; edits to souls, checkboxes, card moves |
| Web server | Event bus | Stream live events to the browser |
| Workflow | Messaging | Deliver answers, notices ("PR #128 opened", "main moved") to agents |
| Workflow | Git | Switch, commit and merge in an agent's worktrees for its tool calls; check a hand-off branch, detect integration; on the CEO's actions, merge to main, push a branch for a PR, or Sync a reserved repo (Git epic) |
| Workflow | Forge adapter | On the CEO's Create PR: open, then poll, comment on and close pull requests |
| Workflow | Home & config, Wiki | Gates and checks from `project.toml`; `contribute` notes into the project's inbox; team files changed by the Chief of Staff's tools |
| Messaging | Instance manager | Deliver a message when the recipient is idle; start a stopped recipient |
| Agent link | Messaging | `ask`, `reply`, `handoff` tool calls |
| Agent link | Workflow | `switch`, `commit`, `merge`, `abort_merge`, `discard`, `set_submodule`, `escalate`, `handoff`, `contribute`, `request_merge`, and the Chief of Staff's tools |
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
│     │             switch · commit · merge (git writes, run by Foreman on the host)  │
│     │  hooks: "foreman-agent hook stop | notification | …"                          │
│     ▼                                                                               │
│  foreman-agent ──────────── tcp: host.containers.internal:<agent port>, token ──────┼──▶ Agent link
│                                                                                     │
│  Mounts                                                                             │
│   ~/.foreman/sandboxes/ledger/tove/ledger-api  Tove's worktree (wt/tove/*), rw      │
│   ~/dev/ledger-api/.git                  the repo every worktree shares, ro         │
│   ~/.foreman/sandboxes/ledger/tove/project  the live project repo (project.toml,    │
│                                          wiki), from ~/.foreman/projects/ledger:    │
│                                          rw for the lead, else ro                   │
│                                          (its .git ro for every agent)              │
│   $CLAUDE_CONFIG_DIR     the instance's private config: prompt, MCP + hooks config, │
│                          history, memory (rw)                                       │
│   …/profiles/<name>/     the project's profile's login file (the CEO's; rw)         │
│   ~/.gitconfig           ro, if it exists                                           │
│  Never mounted: the rest of ~ (~/.ssh, ~/.config, …), the rest of ~/.foreman/       │
│  (secrets, state, teams), the CEO's working files, other agents' worktrees          │
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

**Phasing:** phase 1 builds one image for every project (from `pclaude`'s Dockerfile; `foreman-agent` is mounted, not built in); the four-layer chain comes later ([Agent images](./tech-stack.md#agent-images)). Phase 1 implements `Ensure`, `Refresh` (no layer: the one image) and `Status`; `Prune` and per-layer Refresh come with the chain.

```go
type Images interface {
    Ensure(ctx, project) (ref string, err error) // builds or tags the chain as needed: base → harness → global → project
    Refresh(ctx, layer) error                    // rebuild without cache (new harness versions), then everything below
    Prune(ctx) (reclaimed int64, err error)      // Foreman's outdated images only
    Status(ctx) []ImageStatus                    // Settings → System → Maintenance
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
type Adapter interface {
    // What to run: image, command, env, and the files written into the
    // instance's private config dir: the session prompt (CLAUDE.md / AGENTS.md),
    // the MCP config pointing at foreman-agent, the hooks config.
    Launch(Instance) (runner.Spec, []File, error)
    // How to type a message into the running TUI (text, then Enter; bracketed paste…).
    Deliver(t Terminal, message string) error
    // What a hook event means: idle, busy, waiting for the CEO (a dialog, a question), done.
    Interpret(HookEvent) Status
    // The same hook event as one line for the CEO ("Bash: go test ./..."), or "".
    Describe(HookEvent) string
    // Recognises a crash or rate limit from the exit, for a blocked request.
    Diagnose(exitCode int, tail []byte) Diagnosis
}
```

Details: [Harness adapters](./harness-adapters.md), [Session prompt](./session-prompt.md).

### 4. Agent link (`agentlink` + `foreman-agent`)

How an agent inside a container talks to Foreman. Structured only: Foreman never infers events by reading terminal output, except inside a harness adapter as a last resort for a harness without hooks.

- **`foreman-agent`**: a tiny static binary, mounted read-only into every container when it starts (decided 2026-10-15): Foreman carries a Linux build for each CPU type (amd64, arm64) inside its binary and mounts the one for its own, so it always matches the running Foreman, and upgrading Foreman never rebuilds the image. The harness starts `foreman-agent mcp` as a local (stdio) MCP server, which every harness supports; hooks run `foreman-agent hook <event>`. It relays both to Foreman.
- **The agent port**: **one** port that Foreman opens on the host's loopback, separate from the UI's. Every container connects to it through Podman's host alias (`host.containers.internal`); none exposes a port. Each instance has its own token (container environment), which is its identity: an agent can't speak for another.
- `foreman-agent` reaches Foreman through one address (`tcp://…` today), so another transport (a mounted Unix socket on Linux, say) is a change in one place.

```go
// What the agent link hands to the rest of Foreman, already authenticated:
type ToolCall  struct { Instance; Tool string; Args json.RawMessage }   // → Messaging / Workflow
type HookEvent struct { Instance; Event string; Data json.RawMessage }  // → Instance manager
type Tokens interface { Register(instance) (token string); Revoke(instance) }  // used by the Instance manager
```

**The team-tools router** (`internal/teamtools`) sits between the agent port and the blocks: it knows the caller from its token (instance, deployment, team, lead or not), serves exactly the tools `prompt.Offered` gives the caller's role (a test with the real blocks keeps the two in step), refuses a call from a stopped deployment, and hands each call to the block that owns the tool: Messaging (`ask`, `reply`, `read_message`) or Workflow (`switch`, `commit`, `merge`, `abort_merge`, `handoff`, `contribute`).

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

Built so far (`internal/instances`): `Deploy`, `Stop`, `List`, `Terminal`, `Restore` (re-attach after a Foreman crash, keeping a live agent's `idle` or `awaiting`) and `Shutdown` (a clean exit: every instance stopped and asleep); statuses live in the state store and go out on the event bus, so `Status` and `Watch` are the store and the bus; `Hook` turns harness hook events into statuses; `Type` writes into an instance's terminal. Delivery itself lives in Messaging, which reads the status and types: there is no `Instances.Deliver`.

Details: [Agent runtime](./agent-runtime.md).

### 6. Messaging (`messaging`)

Moves messages between agents, and between agents and the CEO: `ask`, `reply`, `handoff` and `contribute` notices, CEO answers. It numbers them (`msg-N` per deployment), stores them, queues them per recipient and delivers them when the recipient is idle: one plain line typed into its terminal (`Instances.Type`), then Enter after a short pause, so the harness doesn't take the Enter as part of a paste. Every harness so far takes a typed line and Enter, so there is no per-harness `Deliver` yet: an adapter gets one when its harness needs other keystrokes. Messaging hears every status change and deployment stop straight from the instance manager, not through the event bus (which drops a subscriber that falls behind), and sweeps once at startup. Built: `internal/messaging` (`ask`, `reply`, `read_message`, and `Send`, which Workflow uses for hand-offs and contributions). Every delivery also produces a **marker** for the UI (shown beside the terminal, never written into it). A message to an agent that isn't running asks the instance manager to start it. It knows nothing about git, PRs or approvals.

```go
type Messaging interface {
    Send(Message) (ID, error)          // from an agent's tool call, or from the CEO
    Log(deployment) []Message          // the Activity panel
}
```

Details: [Team tools](./team-tools.md), [Team message](../models/team-message.md). Schema: [Protocol → Messages](./protocol.md#messages).

### 7. Workflow (`workflow`)

Gives events their meaning: `switch`, `commit` and `merge` → git writes in the caller's worktrees, through Git; `escalate` and harnesses waiting for the CEO → a [request](../features/requests.md) on the Dashboard (and its answer back); `handoff` → a [work item](../models/work-item.md); `contribute` → a note in the project's `wiki/inbox/` and a notice to the lead; `request_merge` → checks and a [merge request](../models/merge-request.md) with its review; the CEO's decisions on a review (Merge, Request changes, Close; later Create PR) → Git, Messaging and the Forge adapter; project gates; crashes → blocked requests; the Chief of Staff's tools → team files and config changes, on a branch that comes back through `request_merge`.

```go
type Workflow interface {
    Handle(ToolCall) (Result, error)   // switch, commit, merge, abort_merge, discard, set_submodule, escalate, handoff, contribute, request_merge, Chief of Staff tools
    Resolve(request, answer) error     // the CEO answers a request
    Requests(filter) []Request
}
```

Built so far (`internal/workflow`): `switch`, `commit`, `merge`, `abort_merge`, `discard` and `set_submodule` in the caller's worktrees, keeping submodules at the branch's pins ([Sandboxes and git policy → Submodules](./sandboxes-and-git.md#submodules)) (and, for the lead's `commit`, the live project repo), each authored as the agent (`<Name> <agent.invalid>`); `handoff` (refuses uncommitted work, checks the branch, sends the hand-off through Messaging; checks and work items come with the Git epic); `contribute` (a note in `project/wiki/inbox/`, committed as the agent, and a notice to the lead). Requests, gates and merges come later.

Details: [Requests](../features/requests.md), [Pipeline and merges](../features/pipeline-and-merges.md). Schema: [Protocol → Requests](./protocol.md#requests), [work items and merge requests](./protocol.md#work-items-and-merge-requests).

### 8. Git (`git`)

Everything Foreman does with repositories on the agents' behalf, on the host. Each instance gets a **git worktree** of each code repo, sharing the repo with the CEO's checkout and every other agent ([Sandboxes and git policy](./sandboxes-and-git.md)). Agents only read git: each repo's `.git` is mounted read-only, and every git write is a team tool that Foreman runs here, with the repo's hooks off and one git command at a time per repo ([Agents and git](./sandboxes-and-git.md#agents-and-git)). Agents never push anywhere.

```go
type Git interface {
    Prepare(instance, repo) (path string, err error) // the instance's worktree of the repo, made if missing
    Switch(instance, repo, branch) error             // switch its worktree to the branch, made from main if missing
    Commit(instance, repo, message) (commit string, err error) // everything changed in its worktree (or the live project repo, for its writers), on its branch, authored as the agent
    Merge(instance, repo, branch) (conflicts []string, err error) // merge a branch into its worktree; conflicts stay as markers
    AbortMerge(instance, repo) error                 // back out of an unfinished merge
    Prune(instance, repo) error                      // its deployment ended: delete its wt/* branches already merged into main
    Contains(repo, branch, into string) (bool, error) // integration detection: is the work in a lead branch?
    MergeToMain(repo, branch) error                  // the CEO's Merge to main: merge the branch into main on the host
    Push(repo, branch, remote) error                 // on the CEO's Create PR (the branch) or Sync (a reserved repo's main), with Foreman's token
    Fetch(repo) error                                // after a merge on the forge, and as Sync's first step: fetch the forge's main into the repo
}
```

**Phasing.** `Prepare` (a worktree and home branch per agent) arrives in epic 1, [Terminal multiplexer](../../backlog/epics/terminal-multiplexer.md); `Switch`, `Commit`, `Merge` and `AbortMerge`, behind the `switch`, `commit`, `merge` and `abort_merge` team tools, arrive with the agent link in epic 2, together with hand-offs that name a branch (nothing to fetch: every worktree shares the repo). `Prune`, `Contains`, `MergeToMain`, `Push` and `Fetch` (merge plumbing) arrive with the [Git epic](../../backlog/epics/git.md).

## The foundation blocks

No open risk here: standard techniques. Each is still behind its own interface.

### Home & config (`home`)

The [home](../models/home.md) and what the CEO owns in it: the [global repo](../models/home.md#the-global-repo) (team files, settings), the projects' [records](../models/project-config.md#the-project-record) (kept in the State store) and each project's [project repo](../models/project-config.md#the-project-repo) (`project.toml`); the checks of the code repos (on load, on every reload and before each start); and the file watcher that notices changes. All TOML.

```go
type Dashboard interface {
    Projects() []Project;  Project(id) (Project, error)
    Checked(id) (Project, error)                     // the project with its code repos checked now (before a start)
    NewProject(name) (Project, error)                // the record and its project repo: project.toml and a starter wiki
    AddRepo(project, name, path) (Project, error)    // an existing code repo; NewRepo makes an empty one at path
    LocateRepo(project, name, path) (Project, error) // a moved repo: the new path must hold its root commit
    RemoveRepo(project, name) (Project, error)       // refused while a team is deployed on the project
    SetProfile(project, profile) (Project, error)
    Sandbox(project, agent) string                   // <home>/sandboxes/<project>/<agent>
    Teams() []Team;        Team(id) (Team, error)
    SaveTeam(TeamConfig, version) (Team, error)      // version-guarded; one commit per save
    Settings() SettingsFile; SaveSettings(Settings, version) (newVersion, error)
    Watch(ctx, notify func(Change)) error            // file watcher: the global repo, projects/, each project.toml (wiki and Containerfiles join with their cards)
}
```

**`notify` is called outside the home's lock**, after a save or a reload has released it, so a subscriber may read the home back (the instance manager reads `max_sessions` on a settings change). Called under it, that read waited on itself and hung every request after it (found 2026-10-18, when a project's profile was chosen). Changes are queued while the lock is held and told on its release.

Details: [Home](../models/home.md), [Project config](../models/project-config.md), [Persistence → File watching](./persistence.md#file-watching).

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
    Subscribe() (<-chan Event, cancel func())        // every event; a subscriber that falls behind is dropped, then refetches
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
- **UI**: screens use only the `ForemanClient` interface. `ui/src/main.tsx`, the UI's composition root, provides the **real client** (HTTP, events, WebSockets). An area whose server side isn't built yet refuses every call with `NOT_BUILT`, and its screen says "Not built yet" ([Full UI on a demo client](../../backlog/epics/full-ui-demo.md#going-live)).

```ts
interface ForemanClient {
  home: HomeArea            // the home's path, projects, teams, settings
  agents: AgentsArea        // instances, …
  openTerminal(instance: string, handlers: PtyHandlers): PtyConnection
  // … one method per API call; the types are generated from the Go structs
}
```

### Chief of Staff

Not a separate block: an ad-hoc session (Harness, Terminal, Execution engine), like the login container, on the `default` profile, with no deployment. Its one tool, `propose_team`, is checked against the team file's invariants and shown to the CEO; Home & config writes it once the CEO confirms it. Details: [Chief of Staff](../features/chief-of-staff.md).

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
 ask(tove,"…") ──▶ MCP ─────────▶ ToolCall ──▶ store msg-41; marker ─▶ Event bus ─▶ UI: "Mara asked Tove (msg-41)"
 ◀── "msg-41" ──────────────────────────────── (non-blocking: Mara keeps working)
                                              Deliver(tove, text) ──▶ waits: Tove idle
                                                                      types one line ──────────▶ "Message msg-41 from Mara (ask): …"
                                                                                                    … works …
 reply(msg-41, out) ◀────────── (the same path, from Tove; delivered to Mara when she is idle) ─────┘
```

**4. An agent needs the CEO** (approval; epic 4)

```
Agent ── escalate(approval, …) ──▶ Agent link ──▶ Workflow: create request ──▶ Event bus ──▶ Dashboard + notification
      (the tool call stays open: the agent is paused)
CEO answers in the UI ──▶ Workflow.Resolve ──▶ the open tool call returns the answer ──▶ the agent resumes
```

**5. Hand-off, integration and a merge** (hand-off from epic 2; work items, integration and merges from the Git and Workflow epics)

```
Member ── switch(branch), then commit(commits) ──▶ Workflow: Git.Switch, Git.Commit in its worktrees
Member ── handoff(lead, branch, summary) ──▶ Workflow: Git.Commit (pending work), policy check, work item (one repo: nothing to fetch)
                                          └▶ Messaging: deliver "⇄ Tove handed off …" to the lead
Lead ── merge(branch) ──▶ Workflow: Git.Merge into lead/<team>/<topic> in its worktree ──▶ Git.Contains → work item integrated
Lead ── request_merge(branch, entry) ──▶ Workflow: checks ──▶ merge request + request "Review" ──▶ Dashboard
CEO, on a review: Merge ──▶ Workflow ──▶ Git.MergeToMain  │  Create PR ──▶ Git.Push ──▶ Forge.OpenPR  │  Request changes, Dismiss ──▶ delivered to the lead
(after Create PR) CEO merges on the forge ──▶ Forge.PR (polling) ──▶ Workflow: merged, lead told, other leads told to merge main in ──▶ Git.Fetch
```

**6. A screen whose backend isn't built yet**

```
Screen ──▶ ForemanClient (real client) ──▶ that area refuses with NOT_BUILT, no request sent ──▶ the screen says "Not built yet"
                                        ╲─▶ the endpoints, once that area's backend lands (its lines in client.ts)
```

## What is proven, and what isn't

| | Block | Status |
|---|---|---|
| | Terminal streaming to the browser, access guard | **Built and tested** (Direct engine) |
| | Terminal snapshots on reload | Built; one rare edge case open (a pending line wrap), parked as polish |
| **A** | Execution engine on Podman: start, attach, resize, re-attach after a Foreman restart, from Go | **Proven on macOS** (2 ms keystrokes; [results](../../backlog/archive/prove-a-podman-engine.md)); Linux not yet run |
| **B** | Harness + engine: one login shared by many concurrent containers | **Proven for Claude Code on macOS**, print mode: the login file alone, bind-mounted into each private config dir, survives 12 sessions refreshing at once ([results](../../backlog/archive/prove-b-shared-login.md)); an interactive session also refreshes mid-session into the shared file ([D](../../backlog/archive/prove-d-claude-delivery-hooks.md)); Linux not yet run |
| **C** | Agent link: a container reaching the host's loopback port on macOS and Linux | **Proven on macOS** ([results](../../backlog/archive/prove-c-agent-port.md)); Linux (pasta) not yet run |
| **D** | Harness (Claude Code): `Deliver` into a busy TUI, and `Interpret` from hooks (idle, waiting for the CEO) | **Proven for Claude Code on macOS** ([results](../../backlog/archive/prove-d-claude-delivery-hooks.md)): short messages arrive once and intact, idle or mid-turn; questions are signalled (`AskUserQuestion`, then `Notification`); long tool calls go to the background after 120 s. Multi-line text must not be pasted (it arrives wrapped as pasted content) |
| **E** | Harness: prompt file + stdio MCP config accepted by Claude Code, Pi, Hermes | **Proven for Claude Code on macOS** ([results](../../backlog/archive/prove-e-prompt-and-mcp-config.md)); Pi and Hermes when set up |

A–E are proven before the blocks that depend on them are built: one small experiment each, run on the CEO's macOS and Fedora machines through a `just` recipe, with a yes/no outcome written back into the specs ([Prove epic](../../backlog/epics/prove.md)).

## Code today, against these blocks

- `internal/runner` + `runner/direct` + `runner/podman`: the Execution engine and its Direct and Podman implementations, each with its image builder; `images/`: the embedded Containerfile.
- `internal/terminal`: the Terminal block (one process, its log, its views). Its snapshot is the log's tail until `internal/screen` is wired in ([Terminal fidelity](../../backlog/terminal-fidelity.md)).
- `internal/home`: Home & config (`home.Dashboard`): the home's layout (its lock is `internal/userdata`'s), the projects' records (with `store.Projects`) and their code repos' checks, `project.toml`, team files and settings (TOML, validated), the global repo, version-guarded saves (one commit each), the file watcher, New project, and adding, creating, locating and removing code repos.
- `internal/events`: the Event bus; `internal/git`: Foreman's own git, the Git block so far: `Init`, `Commit` and `CommitAs` (named files only), `Prepare(repo, worktree, branch, base)` (the instance-level `Prepare(instance, repo)` above wraps it with the Instance manager; it repairs a worktree whose repo was located elsewhere), `RootCommit`; and, on a worktree, `Switch`, `CommitAll` (finishes a merge once no conflict markers are left), `Merge` (conflicts left as markers), `Merging`, `AbortMerge`, `Clean`, `Branch`, `HasBranch`, `Ahead`. Workflow names the worktree, so the instance-level signatures above are Workflow's view.
- `internal/instances`: the Instance manager.
- `internal/setup`: the setup checklist and the engine's watcher: reaches Podman whenever it is there, restores the instances the first time, builds the agent image in the background, and holds deploying and waking until then ([First run](../features/first-run.md)).
- `internal/harness` (the `Adapter` interface: `Launch`, `Login`, `Connected` so far) with `harness/claudecode` and `harness/fake`; `internal/prompt`: the session prompt composer.
- `internal/store`: the State store (`state.db`, embedded migrations); `Projects`, `Deployments` and `Instances` so far, opened at startup.
- `internal/agentlink` (the agent port: tokens, the tool and hook relay, and the client), `cmd/foreman-agent` (the stdio MCP server and hook forwarder), `agentbin/` (its Linux builds, embedded in `foreman`; the one for Foreman's CPU type is written to the user's cache folder to be mounted, and copies no Foreman has started with for a week are removed): the Agent link. `internal/teamtools`: the team-tools router; `internal/messaging` and `internal/workflow`: the blocks behind it.
- `cmd/foreman/sessions.go`: the harnesses' login terminals, by id (instances' terminals are the Instance manager's).
- `internal/screen` (parked, see [Terminal fidelity](../../backlog/terminal-fidelity.md)), `internal/sessionlog`: the Terminal's `Screen` and `Log`.
- `internal/server`, `internal/access`, `ui/` (with `ForemanClient` and its real implementation): the Web server and UI.
