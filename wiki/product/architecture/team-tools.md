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

- Messages are delivered into the target's PTY as a clearly marked line, e.g. `⇄ Mara asked (req-41): …`.
- Replies come back the same way, `⇄ Rhea replied to req-41`, plus the output. The prompt instructs agents to resume the task that asked for it ([Session prompt](./session-prompt.md)).
- If the target is busy, the message is queued and delivered at its next idle prompt ([Harness adapters](./harness-adapters.md#idle-and-waiting-detection)). If the target is stopped, the requester gets an error line and Foreman raises a blocked request.
- Team messaging is scoped to a deployment. `nyx@ledger` cannot message `nyx@relay`. Cross-deployment messaging is deliberately disallowed.
- Each message gets a monotonically increasing `req-N` **per deployment**, so the UI always qualifies it with the deployment.
- **Delivered text is untrusted data.** A reply's `output` (or a finding's text) lands in another agent's session, where it acts as prompt input, so a confused or compromised agent could try to instruct its teammates. Every injected message is framed as quoted data with its sender (`⇄ Rhea replied to req-41:` followed by the quoted output) and capped in length (longer outputs are truncated with a note pointing at the artifact); nothing is ever delivered across deployments ([Security](./security.md)).

### Line conventions in terminals

Lines Foreman writes into sessions, and their glyphs (the UI colours them, see [Sessions](../features/sessions.md)):

| Glyph | Meaning | Examples |
|---|---|---|
| `⇄` | team-message traffic (violet) | `⇄ ask → Rhea (req-41): …` (requester side) · `⇄ Mara asked (req-41): …` (target side) · `⇄ reply → Mara (req-41)` · `⇄ Rhea replied to req-41: …` · `⇄ handoff → Mara: wt/tove/LED-44` · `⇄ Tove handed off wt/tove/LED-44` |
| `ⓘ` | System note | `ⓘ Your definition changed: …` (a [context update](./session-prompt.md#updates)) · `ⓘ Foreman restarted, output may be missing` |
| `⚠` | Escalation | `⚠ escalate: … [y/N]` |
| `✓` / `✗` | Success / error | `✓ CEO answered: <answer>. Resuming where I paused.` · `✗ No teammate "X" on ledger.` |

A workspace setting toggles the display of team-message marker lines: it hides the lines Foreman injects into the view, never the harness's own output ([Workspace settings](../models/workspace-settings.md#schema)).

## Transport

The tools are an **MCP server** exposed by Foreman; each harness is configured to connect to it ([Harness adapters](./harness-adapters.md)).

- Harnesses run in containers ([Session runner](./session-runner.md)), so the server must be reachable from inside them: at `host.containers.internal` (works with `podman machine` on macOS and with pasta networking on Linux).
- That conflicts with "bind to localhost only". **Design:** a separate **team-tools listener** for containers, distinct from the UI/API listener. It serves only the MCP endpoint (and harness hook callbacks), and every call must carry a **per-instance bearer token**, generated at spawn and passed in the container's environment. The token *is* the caller's identity: the router never trusts a "from" field.
- Alternative: a Unix socket mounted into each container (identity = socket, no network listener). It doesn't cross the macOS VM boundary, so it can't be the only transport.

### Per-instance setup

Nothing about the team tools is configured by the CEO. At every spawn, Foreman:

1. Generates the instance's bearer token: its identity, scoped to its deployment, kept only in memory and in the instance's environment, and revoked when the instance stops.
2. Writes the harness's MCP configuration into the instance's private config directory ([Session runner → Credentials](./session-runner.md#credentials)): one server, `foreman`, at the team-tools listener's URL, with the token in the `Authorization` header (Claude Code: an `--mcp-config` file; Codex: `mcp_servers` in `$CODEX_HOME/config.toml`).
3. Offers only the tools the role allows: `open_pr` is listed for the lead only; the others for everyone ([Tools](#tools)).

Agents get no other MCP servers in v1: only the team tools, plus whatever CLIs their [image](./tech-stack.md#agent-images) contains. CEO-configured MCP servers (per team or workspace) are a possible later addition.

## Pausing on approval

`escalate(approval)` "pauses the caller until answered", and "approval answers also resume the paused tool call". Two ways to build it:
- **Blocking tool call** (the design): the MCP call doesn't return until the CEO resolves the request; the answer is the tool result, so the agent resumes naturally from the same point.
- **Return immediately**, then write the answer into the agent's stdin when resolved: used only as the fallback below.

Harnesses time out long MCP tool calls (e.g. Claude Code's MCP tool timeout), and the CEO may take hours. So the call blocks with a timeout below the harness limit and keep-alive progress notifications where supported; on timeout, return "pending, the answer will arrive in this session" and fall back to stdin injection. Each harness's timeout is measured in the [walking skeleton](../../backlog/walking-skeleton.md).

## Open questions

1. **Transport reachability** (to verify in the [walking skeleton](../../backlog/walking-skeleton.md)): confirm `host.containers.internal` reaches a host listener on Linux (rootless pasta) and macOS (podman machine), and on which address the team-tools listener must bind so that it is reachable by containers but not by the LAN.
