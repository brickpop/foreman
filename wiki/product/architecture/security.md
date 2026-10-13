---
type: spec
status: planned
tags: [architecture, security]
---

# Security

Foreman runs autonomous coding agents with network access on the CEO's machine and serves a UI that can type into their terminals. This entry is the threat model: what is protected, how, and what is knowingly not.

v1 has **no user authentication** (one CEO, local machine). That makes the HTTP surface the most sensitive part: **anyone who can reach the API can type into an agent's terminal, which is code execution as the CEO's user** (inside a container, but with the project's code and the harness's model credentials).

## Assets

- The CEO's machine and home directory (credentials in `~/.config`, `~/.ssh`, the keychain).
- Forge tokens and harness credentials ([Secrets](../models/secrets.md)).
- Project code and `main` of every repo.
- The integrity of decisions: only the CEO merges to `main` and answers approvals.

## Controls

### Access token (decided)

Listening on `127.0.0.1` alone is not enough: any website open in the CEO's browser can send requests and open WebSockets to it, and the agents' containers can reach the host. So every HTTP request and WebSocket upgrade needs a token, on localhost too (Jupyter-style):

1. On first start Foreman generates an unguessable per-install token and stores it in `.foreman/` (0600, never mounted into containers).
2. On every start it prints the URL with the token, `http://127.0.0.1:7420/?token=…`, and opens it in the default browser (`--no-open` skips that).
3. The server swaps the URL token for an HttpOnly, `SameSite=Strict` cookie and redirects to the same URL without it, so the token doesn't stay in the address bar or the history.
4. Missing or wrong token: `401` (and a page that says to use the URL printed in the terminal). Rotating the token (a CLI flag) logs every browser out.

The same token protects "Listen on: Local network"; the Settings warning stays, since anyone holding the URL has full control.

### HTTP hardening

| Threat | Control |
|---|---|
| **LAN access.** "Listen on: Local network" gives everyone on the network interactive shells | The [access token](#access-token-decided) is required on every request; the Settings warning stays |
| **CSRF from any website.** A page the CEO visits can POST to `http://localhost:7420` | Reject state-changing requests whose `Origin` isn't Foreman's own; require a JSON content type; plus the access token |
| **DNS rebinding.** A malicious domain re-resolves to 127.0.0.1 and reads the API as same-origin | **Host header allowlist** (`localhost`, `127.0.0.1`, `[::1]`, plus configured LAN names) on every request |
| **WebSocket hijacking.** Browsers don't apply same-origin policy to WebSockets | Check `Origin` (and the token) on every upgrade to `/ws/pty/*` |
| **Another local user or a container** calling the API | The [access token](#access-token-decided) |

### Agent isolation

Decided: every instance runs in a rootless Podman container ([Session runner](./session-runner.md)).
- Mounted: only the instance's own sandbox (rw) and the project wiki (rw for the scrum master or lead fallback, else ro).
- Never mounted: the home directory, `~/.config`, `~/.ssh`, the keychain, `.foreman/` (secrets, state, team files, the git mirrors), other sandboxes, the CEO's project repos.
- **Secrets are never mounted into containers.** A harness credential is passed as an environment variable to its own container only.
- **Forge tokens are used only by Foreman's push path** ([Forge adapters](./forge-adapters.md)); no agent ever receives one.
- **Branch policy** is enforced where agents can't reach it: in Foreman, on a Foreman-owned mirror, never by hooks inside a sandbox (which the agent controls) ([Sandboxes and git policy](./sandboxes-and-git.md#where-the-policy-is-enforced)).
- **ACP identity** comes from the per-instance token, never from a field the caller sends ([ACP](./acp.md#transport)). ACP is scoped to one deployment.

### Chief of Staff

The Chief of Staff can edit project config, create teams and hire agents. Its edits to project config always land on a branch (`foreman/config-N`) and come back as a review request; it never merges. Changes to Foreman's own files (team definitions) are visible in the UI. Its powers and sandboxing are owned by [Chief of Staff](../features/chief-of-staff.md).

## Accepted limitations (v1)

- **Network exfiltration.** Containers need the network for model APIs, so an agent can send project code anywhere. An egress allowlist per harness is future work.
- **Secrets in PTY logs.** Session logs are raw bytes; anything a harness prints (an env dump, a token in an error) is persisted. Known Foreman secrets are redacted on write; the rest is accepted ([Secrets](../models/secrets.md#open-questions)).
- **Prompt injection between agents.** A reply's `output`, a handoff summary or a finding is text written into another agent's session. A confused or compromised agent (or a challenge team probing the build team) can inject instructions. Mitigations: injected text is framed as quoted data with its sender, length-capped, never crosses deployments ([ACP](./acp.md#open-questions)). The real backstop is that nothing reaches `main` without the CEO.
- **Wiki as a shared channel.** Agents read the wiki that the scrum master writes; it is a prompt-injection path inside a deployment, and across deployments on the same project. Accepted: the wiki is the project's shared memory by design.
- **Model provider sees the code.** Inherent to cloud harnesses.

## Open questions

1. **Egress policy.** Restrict container network to the harness's API hosts (and package registries for checks)? Recommendation: not in v1; record as future.

Harness credential scope is owned by [Session runner](./session-runner.md#open-questions); provider terms by [Harness adapters](./harness-adapters.md#open-questions).
