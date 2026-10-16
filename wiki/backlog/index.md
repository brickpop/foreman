# Backlog

**Current epics, side by side:** [Prove](./epics/prove.md) (the CEO runs the experiments that settle the five runtime facts) and [Terminal multiplexer](./epics/terminal-multiplexer.md). [Full UI on a demo client](./epics/full-ui-demo.md) is done: the UI went live on 2026-10-15, and what the server doesn't serve yet says "Not built yet".

The epics, in order:
0. [Prove](./epics/prove.md) and, alongside it, [Full UI on a demo client](./epics/full-ui-demo.md) (done)
1. [Terminal multiplexer](./epics/terminal-multiplexer.md)
2. [Messaging and hand-offs](./epics/messaging-handoffs.md) and, alongside it, [Install and first run](./epics/install-first-run.md)
3. [Git](./epics/git.md)
4. [Workflow with the CEO](./epics/workflow.md)
5. [Chief of Staff](./epics/chief-of-staff.md)

Cards live in this folder (one `type: task` file each, naming its epic in its `epic:` field); retired ones in `archive/`. This page doesn't list them: browse the **Epics** and **Backlog** boards, or query them:

```sh
wiki --root wiki list --where epic=prove                                               # the cards of an epic
wiki --root wiki list --where type=task --where status!=done --prefix backlog/          # everything live
wiki --root wiki list --where type=task --where tags=decision --where status!=done      # decisions waiting on the CEO
wiki --root wiki list --where type=task --where status=todo --where priority=high       # what's next
wiki --root wiki list --where type=task --where status=in-progress                      # in flight
```

See `WORKFLOW.md` at the project root for the conventions.

## Not yet scheduled

Raised during design, no cards yet; they get one when an epic needs them.

- **What agent "skills" mean technically**: plain text in the session prompt, or the harness's own skills feature; it decides what the Agent page's skill chips actually change.
- **Resource lifecycle**: cleanup of logs, images, agent memory and stopped deployments, and what happens when the laptop sleeps or the Podman VM wakes; without it, disk use grows forever and agents may stall after a sleep.
- **Config changed outside Foreman, at startup** (the CEO's idea, low priority): a CLI prompt when the server starts, Keep mine / Take theirs, for team or settings files changed outside Foreman. What exactly it compares (uncommitted edits in the workspace repo?) is to be defined.
- **Put done agents back to sleep** (the CEO's idea): an agent that is done and waits for nothing exits in a way that can be resumed, back to `asleep`, so it holds no container or slot; waking it resumes its conversation. Needs idle detection (epic 2) and native resume per harness ([Agent runtime → Lazy starts](../product/architecture/agent-runtime.md#lazy-starts)). Built so far (2026-10-16): a clean exit (`/exit`) already goes to sleep and wakes with its conversation; what is left is Foreman deciding on its own that an agent is done.
  - **Design (discussed 2026-10-16, for later):**
    - **Foreman stops the container, never types `/exit`.** `/exit` is specific to Claude Code and could land on the CEO's unsent draft or a dialog. Stopping the container works for every harness, and since 2026-10-16 the container is gone once a stop returns. A new instance-manager `Sleep(id)` marks the agent asleep itself, so the SIGTERM exit code isn't counted as a crash. Nothing is lost: the transcript is written as the agent goes, and only an agent with no turn in progress is stopped.
    - **Only after a clean idle.** When the harness reports idle, `foreman-agent` (in the container, already handling the hooks) lists the container's processes. The idle is clean when nothing is left but the harness and its own helpers. Anything else (a dev server, a test watcher, a build the agent left in the background) keeps the container. Foreman checks the processes rather than asking the agent through a tool, because models forget or misjudge what they started. When it's wrong, it's wrong the safe way: an agent with a stray process just never sleeps.
    - **Foreman sleeps the agent** after N minutes whose last report is still that clean idle, with no messages waiting for it and no unsent CEO draft. Any activity cancels the countdown.
  - **Findings (the CEO, a Claude Code agent idle in Podman):** `PID 1 claude --dangerously-skip-permissions --mcp-config …` and `PID 33 /opt/foreman/foreman-agent mcp`, nothing else. The other processes in the listing were the CEO's own `podman exec` shell and its `ps`, on a second terminal (`pts/1`). So for Claude Code, clean = the harness and its MCP server only. Each harness adapter names its helpers.
  - **Edge cases:**
    - A CEO shell opened in the container (`podman exec`) is a process too: it would keep the agent awake. That's arguably right, since someone is in there.
    - Claude Code's transient children (ripgrep, a hook command) appear only during a turn, and then the agent isn't idle anyway.
    - On the Direct engine (the fake harness) there is no container to inspect, so agents there never sleep.
    - A lead waiting on a teammate's reply sleeps too, and the reply wakes it.
    - The CEO watching a pane sees it turn into the asleep view; the Line or Start wakes it.
  - **Decided (the CEO, 2026-10-16):** on by default, after **20 minutes** of clean idle; a workspace setting, 0 = never. A wake after the prompt cache has expired re-reads the conversation at full price; accepted.
- **Containers when Foreman stops**: a setting to stop or remove the agents' containers when Foreman shuts down, instead of leaving them running for the next start to resume ([Agent runtime → Restarts](../product/architecture/agent-runtime.md#restarts)).
- **CI pipeline and how Foreman is installed** (release binaries, Homebrew): tests on every push, and a setup that doesn't require building from source. The binary itself (everything embedded, both CPU types) is the [Release build](./release-build.md) card.
- **History and debugging views** (past requests, past messages): when an agent misbehaves, the CEO needs to look back at what happened.
- **Hide git from agents entirely**: mount only the worktree, no `.git`, so agents can't read other branches; they would lose `diff`, `log` and `blame` unless Foreman offered them as tools ([Sandboxes and git policy → Agents and git](../product/architecture/sandboxes-and-git.md#agents-and-git)).
- **Review for `project.toml` edits by the lead**: it can change it, gates included, by committing in the live project repo, while the Chief of Staff's changes need review; accepted for v1 ([Security](../product/architecture/security.md#accepted-limitations-v1)).
- **A worktree of the project repo per lead**: several leads (one per team deployed on a project) share the one live `project/` checkout, so a lead's `commit()` can sweep another lead's unfinished edit into its own commit, and only the attribution suffers. Option: give each lead its own worktree and have Foreman merge each commit into `main` ([Sandboxes and git policy → Project knowledge base](../product/architecture/sandboxes-and-git.md#project-knowledge-base)).
- **Merge to main and the CEO's own checkout**: Foreman's Merge to main moves `main` on the host while the CEO's checkout may have `main` checked out; how the two relate is to be decided ([Pipeline and merges → Open questions](../product/features/pipeline-and-merges.md#open-questions)).
