---
type: task
status: done
priority: medium
tags: [feature]
epic: git
---

# The lead fetches upstream: `fetch()`

**Why:** the CEO, 2026-11-04. Foreman never fetches a code repo, and agents can't (`.git` is read-only for them). What someone pushed upstream reaches a team only when the CEO pulls in their own checkout. A lead needs to see it, and merge it in, without waiting for that.

**Decided (the CEO, 2026-11-04):** a team tool, **the lead's only**. It updates the remote-tracking branches (`origin/*`) and nothing else: targets (local `main`, …) never move, as only the CEO's merges and pulls move them. Spec: [Team tools](../../product/architecture/team-tools.md).

**Done when:** the lead's `fetch()` fetches every remote of each of its project's code repos on the host, and its result says what moved; the team reads `origin/*` and merges it in with `merge("origin/main")`; [Team tools](../../product/architecture/team-tools.md) and [Protocol](../../product/architecture/protocol.md) (a `fetch` section) describe it as built.

**How it runs, and why:**
- **The CEO's credentials, safely:** Foreman runs git on the host as the CEO, so their credential helper, SSH agent and keychain apply. The remote URLs come from the repo's `.git/config`, which only the CEO writes: an agent can't redirect a fetch.
- **Never waits on a prompt:** `GIT_TERMINAL_PROMPT=0` and SSH in batch mode, so a missing credential fails the call with git's message instead of hanging the host.
- **No submodules:** `--no-recurse-submodules`. Their URLs are agent-chosen, and fetching them stays with `set_submodule`, credential-free ([Sandboxes and git policy → Submodules](../../product/architecture/sandboxes-and-git.md#submodules)).
- **Hardened like the review diff:** hooks off already (`run`); git's own settings that run programs on a fetch come only from the CEO's config.
- **One at a time per repo** (the existing per-repo lock), with a timeout.

**Decided (the CEO, 2026-11-04):** every remote (`git fetch --all`), no tags (`--no-tags`: the CEO's tags stay their own), and always pruned (`--prune`, no argument): a branch deleted upstream disappears from `<remote>/*`, so a stale one never looks live.

**Result text, as built:** one line per repo: `api: origin/main 1a2b3c4..5d6e7f8, origin/feat/x new, origin/old gone upstream`, `up to date`, `no remote` or `failed: <git's message>`, plus a hint to `merge("origin/main")` when something moved; any repo failing refuses the call (`fetch_failed`) with the same lines.

- [x] `Git.FetchRemotes(ctx, dir)` on the host, with the settings above (`internal/git/remotes.go`): also `SSH_ASKPASS_REQUIRE=never`, `GCM_INTERACTIVE=never`, no askpass programs, its own session (no terminal to prompt on), no `FETCH_HEAD`, 2 minutes at most
- [x] `merge` takes a remote-tracking branch: today it finds only local ones (`HasBranch` checks `refs/heads/`, so `merge("origin/main")` is `branch_not_found`). Resolve `origin/main` to `refs/remotes/origin/main` explicitly, never by git's own guess, so a local branch named `origin/main` (an agent can't make one, but the CEO could) is never taken for it; [Protocol → `merge`](../../product/architecture/protocol.md#merge) says so. Built: a local branch as `refs/heads/<b>`, else `refs/remotes/<b>`; a local branch wins over a remote-tracking one of the same name, and a tag never wins (it did before: a bare name let git take a tag first)
- [x] the tool: `prompt.teamTools` entry (`leadOnly`, code repos only), its router handler (recorded as a git action in Activity; the `⇄` marker waits for markers, built for no tool yet), the lead's prompt line
- [x] tests: what moved is reported, and a second call says up to date; targets and every local branch untouched; a member is never offered it, nor is a project with no code repo; a remote needing a password fails at once with the message (no hang); a submodule's remote isn't fetched; an unreachable remote fails that repo only, the others still fetched
