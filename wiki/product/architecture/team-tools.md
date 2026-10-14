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
| `reply(request_id, output, artifact?)` | the addressee | Answers a request. **Only `output` (and an optional artifact reference) is returned** to the requester. No other context crosses sandboxes. An `artifact` is a branch name or a wiki path, nothing else: paths inside the replier's sandbox are unreadable to the requester by design. |
| `handoff(teammate, branch, summary)` | all | Passes finished work on (usually member → lead). Creates or advances a [work item](../models/work-item.md); never raises a request. The sender's part ends. |
| `escalate(kind, summary, options?, wait?)` | all | Raises a [request](../features/requests.md) to the CEO. `kind ∈ {approval, question, finding}`. `approval` pauses the caller until answered. `question` is non-blocking unless the agent passes `wait: true`. `finding` goes to the CEO only; the owning team's lead hears about it after the CEO triages it ([Requests](../features/requests.md#rules)). |
| `open_pr(branch, title, summary)` | lead only | Creates a [pull request](../models/pull-request.md) from `branch`, one of the lead's own `lead/<team>/*` branches (anything else is an error), carrying the integrated [work items](../models/work-item.md) that branch contains and no other PR has: Foreman runs the project checks, pushes the branch to the forge and opens a PR to `main` (or publishes the branch without a forge). Raises a **review** request ([Pipeline and PRs](../features/pipeline-and-prs.md)). Called again on a branch whose PR is still open, it updates that PR instead (after a rebase or new integrations; [When main moves](../features/pipeline-and-prs.md#when-main-moves)). |

`open_pr` is not offered to members at all (not listed in their MCP tool list), rather than offered and refused.

## Delivery

- Messages reach the target as a **delivery**: text typed into its session through the harness adapter's `Deliver`, only when the target is idle, e.g. `⇄ Mara asked (req-41): …`. Replies come back the same way, `⇄ Rhea replied to req-41:` plus the output. The prompt instructs agents to resume the task that asked for it ([Session prompt](./session-prompt.md)). The exact texts are in [Protocol → Team tools](./protocol.md#team-tools).
- If the target is busy, the message is queued and delivered at its next idle ([Harness adapters](./harness-adapters.md#idle-and-waiting-detection)). If the target is stopped within an active deployment, the message stays queued until it restarts (its stop already raised a blocked request); if the deployment stopped, the message expires. All cases: [Protocol → Messages](./protocol.md#messages).
- Team messaging is scoped to a deployment. `nyx@ledger` cannot message `nyx@relay`. Cross-deployment messaging is deliberately disallowed.
- Each message gets a monotonically increasing `req-N` **per deployment**, so the UI always qualifies it with the deployment.
- **Delivered text is untrusted data.** A reply's `output` (or a finding's text) lands in another agent's session, where it acts as prompt input, so a confused or compromised agent could try to instruct its teammates. Every delivery is framed as quoted data with its sender (`⇄ Rhea replied to req-41:` followed by the quoted output) and capped in length (longer outputs are truncated with a note pointing at the artifact); nothing is ever delivered across deployments ([Security](./security.md)).

### Markers: notes for the CEO, never in the terminal

Anything written into a session's terminal is typed input to the agent ([Protocol → Deliveries and markers](./protocol.md#text-into-a-session-deliveries-and-markers)). So Foreman's notes for the CEO about agent traffic are **markers**: UI events (`instance.marker`) shown **beside** the terminal, in a thin strip on the session pane and in the Messages drawer ([Sessions](../features/sessions.md)), never written into the PTY. The terminal shows only what the agent's program prints, plus the deliveries it receives.

Marker glyphs (the UI colours them):

| Glyph | Meaning | Examples |
|---|---|---|
| `⇄` | team-message traffic (violet) | `⇄ Mara → Rhea · ask · req-41` · `⇄ Rhea → Mara · reply · req-41` · `⇄ Tove handed off wt/tove/LED-44 to Mara` |
| `ⓘ` | System note | `ⓘ Mara's definition changed` · `ⓘ Foreman restarted, output may be missing` |
| `⚠` | Escalation | `⚠ Ilya escalated an approval: new dependency decimal.js` |
| `✓` / `✗` | Success / error | `✓ CEO answered Ilya: approve` · `✗ Delivery to Rhea expired: deployment stopped` |

Deliveries use the same glyphs in their text, because the agent reads them too (`⇄ Mara asked (req-41): …`, `✓ CEO answered: …`, `ⓘ Your definition changed: …`).

A workspace setting toggles the marker strip on the session panes ([Workspace settings](../models/workspace-settings.md#schema)); the Messages drawer always lists the traffic.

## Transport

The tools are an **MCP server** inside Foreman, reached through the [agent link](./building-blocks.md#4-agent-link-agentlink--foreman-agent):

- **`foreman-agent`**, a tiny static binary in every container (built with Foreman), is what the harness talks to. The harness starts `foreman-agent mcp` as a local **stdio** MCP server, the form every harness supports; its hooks run `foreman-agent hook <event>`. `foreman-agent` relays both to Foreman.
- **One agent port** for every container: Foreman opens it on the host's **loopback**, separate from the UI's port. Containers reach it through Podman's host alias, `host.containers.internal`; none exposes a port of its own. Whether the alias reaches the host's loopback on Linux (rootless) and macOS (`podman machine`) is verified in the [Prove epic](../../backlog/epics/prove.md).
- **A token per instance**, passed in the container's environment, is the caller's identity: Foreman never trusts a "from" field, so an agent can't speak for another.
- `foreman-agent` reaches Foreman through **one address** (`tcp://host.containers.internal:<port>` today). Another transport, such as a Unix socket mounted on Linux, is a change in that one place.

### Per-instance setup

Nothing about the team tools is configured by the CEO. At every spawn, Foreman:

1. Generates the instance's token: its identity, scoped to its deployment, kept only in memory and in the instance's environment, and revoked when the instance stops (the Instance manager registers it with the agent link).
2. Writes the harness's MCP configuration into the instance's private config directory ([Session runner → Credentials](./session-runner.md#credentials)): one stdio server, `foreman`, running `foreman-agent mcp` (Claude Code: an `--mcp-config` file). The hooks configuration runs `foreman-agent hook <event>`.
3. Offers only the tools the role allows: `foreman-agent` asks Foreman for its tool list, and Foreman answers by the token's role; `open_pr` is listed for the lead only ([Tools](#tools)).

Agents get no other MCP servers in v1: only the team tools, plus whatever CLIs their [image](./tech-stack.md#agent-images) contains. CEO-configured MCP servers (per team or workspace) are a possible later addition.

## Pausing on approval

`escalate(approval)` "pauses the caller until answered", and "approval answers also resume the paused tool call". Two ways to build it:
- **Blocking tool call** (the design): the MCP call doesn't return until the CEO resolves the request; the answer is the tool result, so the agent resumes naturally from the same point.
- **Return immediately**, then write the answer into the agent's stdin when resolved: used only as the fallback below.

Harnesses time out long MCP tool calls (e.g. Claude Code's MCP tool timeout), and the CEO may take hours. So the call blocks with a timeout below the harness limit and keep-alive progress notifications where supported; on timeout, return "pending, the answer will arrive in this session" and fall back to stdin injection. Each harness's timeout is measured in the [Prove epic](../../backlog/epics/prove.md).

## Open questions

1. **Transport reachability** (to verify in the [Prove epic](../../backlog/epics/prove.md)): confirm that `foreman-agent`, through `host.containers.internal`, reaches the agent port bound to the host's loopback on Linux (rootless pasta) and macOS (podman machine); if the loopback isn't reachable on one of them, which address the agent port must bind to so that containers reach it but the LAN doesn't.
