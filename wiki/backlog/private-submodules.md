---
type: task
status: todo
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
- **The CEO's credentials, only for the URLs on `main`.** A URL in `.gitmodules` on the repo's main branch was put there by a merge the CEO made, so it counts as approved: fetch it with the CEO's own git (SSH agent, credential helper), as Clone already does for a project repo ([Add a project → Effect](../product/features/add-project.md#effect)). A URL only on an agent's branch keeps the current rules. Risk: an agent can still move an approved submodule to any commit of that repo, which is fine, since the CEO already has access to it.
- **An allowlist in Project settings.** The CEO names the remote repos Foreman may fetch with their credentials. More explicit, but one more thing to maintain.
- **Both of the first two:** local first, then the CEO's credentials for URLs on `main`.

**Also, in any case:** a submodule that couldn't be filled at a start is told to the agent (in its first message, like the `your worktrees hold earlier work` notice) and shown to the CEO (a finding, or a line on the Sessions page), with what to do.

**Edge cases:** an scp-like URL (`git@host:org/repo.git`) and an `ssh://` one; a URL relative to the repo's remote (`../platform-doc.git`); nested submodules with `recursive_submodules`; a submodule whose URL on the agent's branch differs from the one on `main` (same name, new URL: not approved); the CEO's checkout never having run `submodule update`; a pin present locally under another submodule name.

- [ ] decide the option with the CEO
- [ ] the fill (and the policy change in [Sandboxes and git policy](../product/architecture/sandboxes-and-git.md#submodules), the [Security](../product/architecture/security.md) spec if credentials are used)
- [ ] tell the agent and the CEO when a submodule can't be filled at a start
- [ ] tests: an ssh submodule on `main` fills; one only on an agent's branch is still refused; a changed URL under an approved name is refused; the agent and the CEO are told on a failure
