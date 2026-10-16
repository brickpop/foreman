---
type: spec
status: partial
tags: [architecture, security]
---

# Security

Foreman runs autonomous coding agents with network access on the CEO's machine and serves a UI that can type into their terminals. This entry is the threat model: what is protected, how, and what is knowingly not.

v1 has **no user authentication** (one CEO, local machine). That makes the HTTP surface the most sensitive part: **anyone who can reach the API can type into an agent's terminal, which is code execution as the CEO's user** (inside a container, but with the project's code and the harness's model credentials).

## Assets

- The CEO's machine and home directory (credentials in `~/.config`, `~/.ssh`, the keychain).
- Forge tokens and harness credentials ([Secrets](../models/secrets.md)).
- Project code and `main` of every repo.
- The integrity of decisions: only the CEO decides what reaches `main` (Foreman merges only on the CEO's Merge to main; nothing goes to a forge without the CEO's Create PR or Sync) and answers approvals.

## Controls

### Access token

Listening on `127.0.0.1` alone is not enough: any website open in the CEO's browser can send requests and open WebSockets to it, and the agents' containers can reach the host. So every HTTP request and WebSocket upgrade needs a token, on localhost too (Jupyter-style):

1. On first start Foreman generates an unguessable per-install token and stores it in `.foreman/` (0600, never mounted into containers).
2. On every start it prints the URL with the token, `http://127.0.0.1:8888/?token=…`, and opens it in the default browser (`--no-open` skips that).
3. The server swaps the URL token for an HttpOnly, `SameSite=Strict` cookie and redirects to the same URL without it, so the token doesn't stay in the address bar or the history.
4. Missing or wrong token: `401` (and a page that says to use the URL printed in the terminal). Rotating the token (`foreman --new-token`, see [Starting Foreman](../models/workspace.md#starting-foreman)) logs every browser out.

The same token protects "Listen on: Local network"; the Settings warning stays, since anyone holding the URL has full control.

### HTTP hardening

| Threat | Control |
|---|---|
| **LAN access.** "Listen on: Local network" gives everyone on the network interactive shells | The [access token](#access-token) is required on every request; the Settings warning stays |
| **CSRF from any website.** A page the CEO visits can POST to `http://localhost:8888` | Reject state-changing requests whose `Origin` isn't Foreman's own; require a JSON content type; plus the access token |
| **DNS rebinding.** A malicious domain re-resolves to 127.0.0.1 and reads the API as same-origin | **Host header allowlist** (`localhost`, `127.0.0.1`, `[::1]`, plus configured LAN names) on every request |
| **WebSocket hijacking.** Browsers don't apply same-origin policy to WebSockets | Check `Origin` (and the token) on every upgrade to `/ws/pty/*` |
| **Another local user or a container** calling the API | The [access token](#access-token) |

### Agent isolation

Decided: every instance runs in a rootless Podman container ([Session runner](./session-runner.md)).
- Mounted: the instance's own sandbox, i.e. its git worktrees (rw), each code repo's `.git` (**read-only**, shared by every agent of the project), its private harness config directory (rw), the live project repo with `project.toml` and the wiki (files rw for the lead, else ro; its `.git` ro for everyone), the harness's shared login folder `.foreman/credentials/<harness>/` (rw) and `~/.gitconfig` (ro, only if it exists) ([Credentials](./session-runner.md#credentials)).
- Never mounted: the home directory, `~/.config`, `~/.ssh`, the keychain, the rest of `.foreman/` (secrets, state, team files), other agents' worktrees, the CEO's working files.
- **The secrets store is never mounted into containers** (forge tokens, the access token). The one credential agents hold is their harness's shared login. A misbehaving agent could log it out or corrupt it for every agent of that harness, which then raises "Reconnect" ([Credentials](./session-runner.md#credentials)). Accepted for v1; per-instance logins would contain it.
- **Forge tokens are used only by Foreman's push path** ([Forge adapters](./forge-adapters.md)); no agent ever receives one.
- **Git belongs to Foreman; agents only read it.** Every repo's `.git` is mounted read-only for every agent, the project repo's included (its only writer, the lead, edits its files and commits through `commit`), and every git write is a team tool (`switch`, `commit`, `merge`, `handoff`) that Foreman runs on the host, with the repo's hooks and fsmonitor turned off ([Sandboxes and git policy → Agents and git](./sandboxes-and-git.md#agents-and-git)). So the **branch policy** holds by construction, never by hooks in the repo ([where it is enforced](./sandboxes-and-git.md#where-the-policy-is-enforced)), and no agent can damage refs or plant a hook or a `.git/config` setting (`core.hooksPath`, `core.fsmonitor`) that would run a program on the host the next time the CEO or Foreman runs git there. Accepted: every agent can read every branch of the repo. Hiding `.git` entirely is a later option.
- **Team-tools identity** comes from the per-instance token, never from a field the caller sends ([Team tools](./team-tools.md#transport)). Team messaging is scoped to one deployment.

Harness credentials are designed in [Session runner → Credentials](./session-runner.md#credentials) (the CEO's host login is shared, the host's harness config directories never are).

### Chief of Staff

The Chief of Staff can edit project config, create teams and hire agents. Its edits to project config (in the project repo) and to team files (in the workspace repo) always land on a branch and come back through `request_merge` as a review request; it never merges. Its powers and sandboxing are owned by [Chief of Staff](../features/chief-of-staff.md).

## Accepted limitations (v1)

- **Network exfiltration.** Containers need the network for model APIs, so an agent can send project code anywhere. Restricting container egress to each harness's API hosts (and package registries for checks) is future work, not v1.
- **Secrets in PTY logs.** Session logs are raw bytes; anything a harness prints (an env dump, a token in an error) is persisted. Known Foreman secrets are redacted on write; the rest is accepted ([Secrets](../models/secrets.md#invariants)).
- **Prompt injection between agents.** A reply's `output`, a handoff summary or a finding is text written into another agent's session. A confused or compromised agent (or a challenge team probing the build team) can inject instructions. Mitigations: injected text is framed as quoted data with its sender, length-capped, never crosses deployments ([Team tools](./team-tools.md#delivery)). The real backstop is that nothing reaches `main` without the CEO.
- **Wiki as a shared channel.** Agents read the wiki that the lead writes; it is a prompt-injection path inside a deployment, and across deployments on the same project. Accepted: the wiki is the project's shared memory by design.
- **Model provider sees the code.** Inherent to cloud harnesses.
- **The lead can change `project.toml` without review.** Its rw mount of the live project repo covers `project.toml`, gates (`always_ask_ceo`) and checks included, and its `commit()` lands on the project repo's `main` without review, while the Chief of Staff's changes to the same file need the CEO's review. Accepted for v1 ("v1 today is better than v1 in 10 years"); tracked in the [backlog](../../backlog/index.md#not-yet-scheduled).
