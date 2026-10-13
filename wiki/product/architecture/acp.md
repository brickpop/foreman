---
type: spec
status: planned
tags: [architecture, acp, agents, requests, security]
---

# ACP

The **agent communication protocol**: the tools Foreman exposes to every harness so agents can collaborate across sandboxes and reach the CEO. The ACP router serves the tools, routes messages between instances of the same [deployment](../models/deployment.md), stores them ([ACP message](../models/acp-message.md)), and injects replies into the requester's session. The rule that makes isolation hold: **only the designated `output` of a request crosses from one sandbox to another.**

## Tools

| Tool | Who | Semantics |
|---|---|---|
| `ask(teammate, request) → request_id` | all | Sends a request to a teammate in the same deployment. Non-blocking: the caller keeps working. |
| `reply(request_id, output, artifact?)` | the addressee | Answers a request. **Only `output` (and an optional artifact reference) is returned** to the requester. No other context crosses sandboxes. |
| `handoff(teammate, branch, summary)` | all | Passes finished work on (usually member → lead). Creates or advances a [pipeline item](../models/pipeline-item.md). The sender's part ends. |
| `escalate(kind, summary, options?)` | all | Raises a [request](../features/requests.md) to the CEO. `kind ∈ {approval, question, finding}`. `approval` pauses the caller until answered. `question` is non-blocking unless the agent chooses to wait. `finding` routes to the CEO, and to the owning team's lead if one is named. |
| `open_pr(title, summary)` | lead only | Asks Foreman to push `lead/<branch>` to the forge and open a PR to `main`. Runs project checks. Raises a **review** request ([Pipeline and PRs](../features/pipeline-and-prs.md)). |

`open_pr` is not offered to members at all (not listed in their MCP tool list), rather than offered and refused.

## Delivery

- Messages are delivered into the target's PTY as a clearly marked line, e.g. `⇄ Mara asked (req-41): …`.
- Replies come back the same way, `⇄ Rhea replied to req-41`, plus the output. The prompt instructs agents to resume the task that asked for it ([Session prompt](./session-prompt.md)).
- If the target is busy, the message is queued and delivered at its next idle prompt ([Harness adapters](./harness-adapters.md#idle-and-waiting-detection)). If the target is stopped, the requester gets an error line and Foreman raises a blocked request.
- ACP is scoped to a deployment. `nyx@ledger` cannot message `nyx@relay`. Cross-deployment messaging is deliberately disallowed.
- Each message gets a monotonically increasing `req-N` **per deployment**.

### Line conventions in terminals

Lines Foreman writes into sessions, and their glyphs (the UI colours them, see [Sessions](../features/sessions.md)):

| Glyph | Meaning | Examples |
|---|---|---|
| `⇄` | ACP traffic (violet) | `⇄ ask → Rhea (req-41): …` (requester side) · `⇄ Mara asked (req-41): …` (target side) · `⇄ reply → Mara (req-41)` · `⇄ Rhea replied to req-41: …` · `⇄ handoff → Mara: wt/tove/LED-44` · `⇄ Tove handed off wt/tove/LED-44` |
| `ⓘ` | System note | `ⓘ CEO gave Tove a direct instruction: …` (in the lead's session) |
| `⚠` | Escalation | `⚠ escalate: … [y/N]` |
| `✓` / `✗` | Success / error | `✓ CEO answered: <answer>. Resuming where I paused.` · `✗ No teammate "X" on ledger.` |
| `❯` | CEO input echo | `❯ <text>` |

A workspace setting toggles the display of ACP marker lines in the UI. Whether they are hidden in the UI only or not injected at all is a question for [Workspace settings](../models/workspace-settings.md#open-questions).

## Transport

The tools are an **MCP server** exposed by Foreman; each harness is configured to connect to it ([Harness adapters](./harness-adapters.md)).

- Harnesses run in containers ([Session runner](./session-runner.md)), so the server must be reachable from inside them: at `host.containers.internal` (works with `podman machine` on macOS and with pasta networking on Linux).
- That conflicts with "bind to localhost only". **Design:** a separate **ACP listener** for containers, distinct from the UI/API listener. It serves only the MCP endpoint (and harness hook callbacks), and every call must carry a **per-instance bearer token**, generated at spawn and passed in the container's environment. The token *is* the caller's identity: the router never trusts a "from" field.
- Alternative: a Unix socket mounted into each container (identity = socket, no network listener). It doesn't cross the macOS VM boundary, so it can't be the only transport.

## Pausing on approval

`escalate(approval)` "pauses the caller until answered", and "approval answers also resume the paused tool call". Two ways to build it:
- **Blocking tool call** (recommended): the MCP call doesn't return until the CEO resolves the request; the answer is the tool result, so the agent resumes naturally from the same point.
- **Return immediately**, then write the answer into the agent's stdin when resolved.

Harnesses time out long MCP tool calls (e.g. Claude Code's MCP tool timeout), and the CEO may take hours. Recommendation: block with a timeout below the harness limit and keep-alive progress notifications where supported; on timeout, return "pending, the answer will arrive in this session" and fall back to stdin injection.

## Open questions

1. **Transport reachability** (spike): confirm `host.containers.internal` reaches a host listener on Linux (rootless pasta) and macOS (podman machine), and on which address the ACP listener must bind so that it is reachable by containers but not by the LAN.
2. **Approval pause mechanism**: blocking tool call vs immediate return + stdin injection (recommendation above). Measure each harness's tool-call timeout.
3. **Untrusted output.** A reply's `output` is written into another agent's session, where it acts as prompt input. A compromised or confused agent (or a challenge team's finding text) can inject instructions into teammates. Mitigation options: always frame injected text as quoted data with its sender, cap its length, never deliver across deployments. Recommendation: framing + length cap in v1; documented in [Security](./security.md).
4. **Artifact references.** What may `artifact` point to (a branch, a path in the replier's sandbox, a wiki entry)? Paths in another sandbox are unreadable by design. Recommendation: a branch name or a wiki path only.

Question blocking and findings routing (PRD §16.5, §16.7) are owned by [Requests](../features/requests.md#open-questions); the `review` message kind by [ACP message](../models/acp-message.md#open-questions).
