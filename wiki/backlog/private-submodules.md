---
type: task
status: in-progress
priority: high
tags: [bug]
epic: git
---

# Private submodules of the CEO's own repos

**Why:** the CEO, 2026-10-27, on `aragon-workspace`. The lead started without the `platform-doc` submodule of `strategy-doc`, and the server log only said:

```
WARN checking out the submodules instance=leady@aragon-workspace repo=strategy-doc
err="git fetch: exit status 1: fatal: transport 'ssh' not allowed
fatal: clone of 'git@github.com:aragon/platform-doc.git' into submodule path
'…/sandboxes/aragon-workspace/leady/strategy-doc/platform-doc' failed …"
```

Foreman is doing what it was designed to do ([Sandboxes and git policy → Submodules](../product/architecture/sandboxes-and-git.md#submodules)): submodule fetches run without the CEO's credentials and over https only, so an `ssh` URL is refused, and so is a private repo over https. The reason is that an agent picks these URLs (`set_submodule`, or by editing `.gitmodules` on its branch), and lending it the CEO's credentials would let it pull any private repo the CEO can reach into its sandbox. But this URL wasn't chosen by an agent: it's on the repo's `main`, and the CEO's own checkout most likely has the submodule already. Private submodules over SSH are common in company repos, so as things stand Foreman can't work on them.

**Two problems:**
1. **The submodule can't be filled.** The agent works without `platform-doc`.
2. **Nobody is told.** At a worktree's creation the failure is only a `WARN` in the server log (`internal/instances/launch.go`). Neither the agent nor the CEO sees it in the UI. The spec says the next `switch` or `merge` tries again "and says so", but the first start says nothing.

**Options to decide with the CEO:**
- **Reuse what the CEO's checkout already has (no network).** When the commit a branch pins is already in the CEO's repo (`<repo>/.git/modules/<name>/`), fill the worktree's submodule from there, as a local reference, instead of fetching. Nothing leaves the machine, no credentials are used, and it covers the usual case: the agent's branch pins what `main` pins. A pin the CEO's checkout doesn't have still needs a fetch.
- **The CEO's credentials, only for the URLs on a target.** A URL in `.gitmodules` on the repo's default branch (or, to decide, any of its [targets](../product/features/pipeline-and-merges.md#merge-targets); but a lead picks the commit a target starts at, any commit of the repo, so a target's first commit isn't always one the CEO merged) was put there by a merge the CEO made, so it counts as approved: fetch it with the CEO's own git (SSH agent, credential helper), as Clone already does for a project repo ([Add a project → Effect](../product/features/add-project.md#effect)). A URL only on an agent's branch keeps the current rules. Risk: an agent can still move an approved submodule to any commit of that repo, which is fine, since the CEO already has access to it.
- **An allowlist in Project settings.** The CEO names the remote repos Foreman may fetch with their credentials. More explicit, but one more thing to maintain.
- **Both of the first two:** local first, then the CEO's credentials for URLs on a target.

**Also, in any case:** a submodule that couldn't be filled at a start is told to the agent (in its first message, like the `your worktrees hold earlier work` notice) and shown to the CEO (a finding, or a line in the project's Session), with what to do.

**Edge cases:** an scp-like URL (`git@host:org/repo.git`) and an `ssh://` one; a URL relative to the repo's remote (`../platform-doc.git`); nested submodules with `recursive_submodules`; a submodule whose URL on the agent's branch differs from the one on `main` (same name, new URL: not approved); the CEO's checkout never having run `submodule update`; a pin present locally under another submodule name.

## Decided (the CEO, 2026-10-30): the https counterpart, then the CEO's checkout

Not the CEO's credentials: an SSH URL is tried as its https counterpart, still without credentials (a public repo answers), and failing that the submodule comes from the CEO's own checkout, `<repo>/.git/modules/<name>`, locally.

- [x] the fill: `update` one submodule at a time, `httpsOf`, then `ceoCopy` with local paths allowed for that one clone, never recursively. The URL `.gitmodules` gives is never used for the fallback, so a changed URL under an approved name gets the CEO's copy, never the new URL (`internal/git/submodules.go`)
- [x] the policy in [Sandboxes and git policy → Submodules](../product/architecture/sandboxes-and-git.md#submodules) and [Protocol → `set_submodule`](../product/architecture/protocol.md#set_submodule). No credential is used, so [Security](../product/architecture/security.md) is unchanged
- [x] tests: the URL forms; a private submodule filled from the CEO's checkout; one the CEO hasn't checked out (the error says what to run there); an agent's `file://` URL never fetched; a nested local URL refused after a fallback (checked to fail without the guard)
- [ ] tell the agent and the CEO when a submodule can't be filled at a start: still only a `WARN` in the server log (`internal/instances/launch.go`)
- [ ] the CEO checks `strategy-doc/platform-doc` fills for Sol: it needs `platform-doc` checked out in the CEO's own `strategy-doc` clone

**Edge cases, as they stand:** a URL relative to the repo's remote (`../platform-doc.git`) is still refused; nested submodules never come from the CEO's checkout, only over https; `set_submodule(url)` still accepts only https.
