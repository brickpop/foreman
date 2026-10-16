---
type: spec
status: planned
tags: [architecture, team-tools, agents, requests, security]
---

# Team tools

The **team tools** are the tools Foreman exposes to every harness, as an MCP server, so agents can collaborate across sandboxes and reach the CEO. (The original PRD called them "ACP"; they were renamed so that ACP only ever means the [Agent Client Protocol](./harness-adapters.md#session-drivers).) The team-tools router serves the tools, routes messages between instances of the same [deployment](../models/deployment.md), stores them ([Team message](../models/team-message.md)), and injects replies into the requester's session. The rule that makes isolation hold: **only the designated `output` of a request crosses from one sandbox to another.**

## Tools

| Tool | Who | Semantics |
|---|---|---|
| `ask(teammate, request) → request_id` | all | Sends a request to a teammate in the same deployment. Non-blocking: the caller keeps working. |
| `read_message(id)` | all | Reads one message addressed to the caller, in full: a delivery only names it and quotes its start ([Delivery](#delivery)). |
| `reply(request_id, output, artifact?)` | the addressee | Answers a request. **Only `output` (and an optional artifact reference) is returned** to the requester. No other context crosses sandboxes. An `artifact` is a branch name or a wiki path, nothing else: paths inside the replier's sandbox are unreadable to the requester by design. |
| `switch(branch)` | all | Switches the caller's worktrees, in each of its code repos, to one of its own branches (`wt/<agent>/<topic>`, or `lead/<team>/<topic>` for the lead), creating it from `main` if it doesn't exist yet. Details: [Protocol → `switch`](./protocol.md#switch). |
| `commit(message)` | all | Commits everything changed in the caller's worktrees, each on the branch checked out there, authored as the caller. For the lead, it also commits its changes in the live project repo. Agents read git but can't write it ([Sandboxes and git policy → Agents and git](./sandboxes-and-git.md#agents-and-git)); Foreman runs the commit on the host. Details: [Protocol → `commit`](./protocol.md#commit). |
| `merge(branch)` | all | Merges a named branch (the lead's, `main`, a teammate's) into the caller's worktrees. Conflict markers stay in the files; the caller resolves them and calls `commit`. Details: [Protocol → `merge`](./protocol.md#merge). |
| `abort_merge()` | all | Backs out of an unfinished merge: the branch and its files are as before it. Details: [Protocol → `abort_merge`](./protocol.md#abort_merge). |
| `handoff(teammate, branch, summary)` | members | Passes finished work on (usually member → lead; the lead, who works only on `lead/<team>/*`, has nothing to hand off). It names a committed branch and never commits: a caller with uncommitted changes is refused (commit first). Creates or advances a [work item](../models/work-item.md); never raises a request. The sender's part ends. |
| `contribute(kind, title, body)` | all | Adds to the project's knowledge base without write access to it. `kind ∈ {finding, proposal, note}`. Foreman writes a note into `project/wiki/inbox/` (the author comes from the caller's token) and tells the lead, who decides what becomes of it. The lead can write the wiki directly, but may use it too. Never raises a request. Details: [Protocol → `contribute`](./protocol.md#contribute). |
| `escalate(kind, summary, options?, wait?)` | all | Raises a [request](../features/requests.md) to the CEO. `kind ∈ {approval, question, finding}`. `approval` pauses the caller until answered. `question` is non-blocking unless the agent passes `wait: true`. `finding` goes to the CEO only; the owning team's lead hears about it after the CEO triages it ([Requests](../features/requests.md#rules)). |
| `request_merge(branch, entry)` | lead only | Asks the CEO to merge `branch`, one of the lead's own `lead/<team>/*` branches (anything else is an error), into `main`; `entry` is a path in the project's wiki (usually the task card) that explains what was done. Foreman runs the project checks and creates a [merge request](../models/merge-request.md) per repo with new commits, carrying the integrated [work items](../models/work-item.md) that branch contains and no other merge request has, and raises a **review** request. Nothing is pushed: the CEO chooses Merge to main, Create PR on the forge, or Reject ([Pipeline and merges](../features/pipeline-and-merges.md)). Called again on a branch whose merge request is still open, it refreshes it (after merging `main` in, or new integrations; [When main moves](../features/pipeline-and-merges.md#when-main-moves)). Details: [Protocol → `request_merge`](./protocol.md#request_merge). |

`request_merge` is not offered to members at all (not listed in their MCP tool list), rather than offered and refused; likewise `handoff` is not offered to the lead.

## Delivery

- Messages reach the target as a **delivery**: one line of plain words, no symbols, typed into its session through the harness adapter's `Deliver`, only when the target is idle: `Message msg-41 from Mara (ask): <its start>… Read it with read_message("msg-41"); answer with reply("msg-41", output).` The agent reads the whole message with `read_message`. Replies come back the same way, and the prompt instructs agents to resume the task that asked for it ([Session prompt](./session-prompt.md)). Why one line (the CEO's decision, 2026-10-15): a multi-line paste reaches Claude Code wrapped as pasted content, which it treats as possibly not the user's ([experiment D](../../backlog/prove-d-claude-delivery-hooks.md)); a long reply never floods the terminal either. The exact texts are in [Protocol → Team tools](./protocol.md#team-tools).
- If the target is busy, the message is queued and delivered at its next idle ([Harness adapters](./harness-adapters.md#idle-and-waiting-detection)). If the target is stopped within an active deployment, the message stays queued until it restarts (its stop already raised a blocked request); if the deployment stopped, the message expires. All cases: [Protocol → Messages](./protocol.md#messages).
- Team messaging is scoped to a deployment. `nyx@ledger` cannot message `nyx@relay`. Cross-deployment messaging is deliberately disallowed.
- Each message gets a monotonically increasing `msg-N` **per deployment**, so the UI always qualifies it with the deployment. The same id reads it (`read_message`) and answers an ask (`reply`).
- **Delivered text is untrusted data.** A reply's `output` (or a finding's text) lands in another agent's session, where it acts as prompt input, so a confused or compromised agent could try to instruct its teammates. Every delivery names its sender, and the message itself only comes back through `read_message`, framed as quoted data from that sender and capped in length (longer outputs are truncated with a note pointing at the artifact); nothing is ever delivered across deployments ([Security](./security.md)).

### Markers: notes for the CEO, never in the terminal

Anything written into a session's terminal is typed input to the agent ([Protocol → Deliveries and markers](./protocol.md#text-into-a-session-deliveries-and-markers)). So Foreman's notes for the CEO about agent traffic are **markers**: UI events (`instance.marker`) shown **beside** the terminal, in a thin strip on the session pane and in the Messages drawer ([Sessions](../features/sessions.md)), never written into the PTY. The terminal shows only what the agent's program prints, plus the deliveries it receives.

Marker glyphs (the UI colours them):

| Glyph | Meaning | Examples |
|---|---|---|
| `⇄` | team-message traffic (violet) | `⇄ Mara → Rhea · ask · msg-41` · `⇄ Rhea → Mara · reply · msg-41` · `⇄ Tove handed off wt/tove/LED-44 to Mara` |
| `ⓘ` | System note | `ⓘ Mara's definition changed` · `ⓘ Foreman restarted, output may be missing` |
| `⚠` | Escalation | `⚠ Ilya escalated an approval: new dependency decimal.js` |
| `✓` / `✗` | Success / error | `✓ CEO answered Ilya: approve` · `✗ Delivery to Rhea expired: deployment stopped` |

Deliveries don't use these glyphs: what the agent reads is plain words (`Message msg-41 from Mara (ask): …`, `From the CEO, on …`, `From Foreman: …`).

A workspace setting toggles the marker strip on the session panes ([Workspace settings](../models/workspace-settings.md#schema)); the Messages drawer always lists the traffic.

## Transport

The tools are an **MCP server** inside Foreman, reached through the [agent link](./building-blocks.md#4-agent-link-agentlink--foreman-agent):

- **`foreman-agent`**, a tiny static binary mounted read-only into every container when it starts (decided 2026-10-15): Foreman carries a Linux build for its own CPU type inside its binary, so it always matches the running Foreman, and upgrading Foreman never rebuilds the image, is what the harness talks to. The harness starts `foreman-agent mcp` as a local **stdio** MCP server, the form every harness supports; its hooks run `foreman-agent hook <event>`. `foreman-agent` relays both to Foreman.
- **One agent port** for every container: Foreman opens it on the host's **loopback**, separate from the UI's port: the next one (`--port` + 1, so 8889 by default). Containers reach it through Podman's host alias, `host.containers.internal`; none exposes a port of its own. Whether the alias reaches the host's loopback on Linux (rootless) and macOS (`podman machine`) is verified in the [Prove epic](../../backlog/epics/prove.md).
- **A token per instance**, passed in the container's environment, is the caller's identity: Foreman never trusts a "from" field, so an agent can't speak for another.
- `foreman-agent` reaches Foreman through **one address** (`tcp://host.containers.internal:<port>` today). Another transport, such as a Unix socket mounted on Linux, is a change in that one place.

### Per-instance setup

Nothing about the team tools is configured by the CEO. At every spawn, Foreman:

1. Gives the instance its token: its identity, signed with the workspace's agent-link secret (`.foreman/credentials/agent-link`, never in git) and passed only in the instance's environment. It is accepted only while the instance runs, so a stopped instance's token is dead; and since the same instance always gets the same token, a container that kept running across a Foreman restart is still recognised (built 2026-10-15, rather than tokens kept only in memory, which a restart would have lost).
2. Writes the harness's MCP configuration into the instance's private config directory ([Session runner → Credentials](./session-runner.md#credentials)): one stdio server, `foreman`, running `foreman-agent mcp` (Claude Code: an `--mcp-config` file). The hooks configuration runs `foreman-agent hook <event>`.
3. Offers only the tools the role allows: `foreman-agent` asks Foreman for its tool list, and Foreman answers by the token's role; `request_merge` is listed for the lead only, `handoff` for members only ([Tools](#tools)).

Agents get no other MCP servers in v1: only the team tools, plus whatever CLIs their [image](./tech-stack.md#agent-images) contains. CEO-configured MCP servers (per team or workspace) are a possible later addition.

## Waiting on approval

`escalate(approval)` (and `question` with `wait: true`) **returns at once** (the CEO's decision, 2026-10-15): the result tells the agent to wait for the CEO's answer before the step it asked about, and the instance is `awaiting` until then. The answer arrives as a delivery like any other message (`From the CEO, on <request>: <answer>. Resume where you paused.`). No tool call is held open: Claude Code moves a call to the background after 120 s ([experiment D](../../backlog/prove-d-claude-delivery-hooks.md)), and one mechanism (deliveries) serves every answer.

## Open questions

1. **Transport reachability** (to verify in the [Prove epic](../../backlog/epics/prove.md)): confirm that `foreman-agent`, through `host.containers.internal`, reaches the agent port bound to the host's loopback on Linux (rootless pasta) and macOS (podman machine); if the loopback isn't reachable on one of them, which address the agent port must bind to so that containers reach it but the LAN doesn't.
