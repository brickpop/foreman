---
type: task
status: todo
priority: high
tags: [feature]
epic: projects-anywhere
---

# Sandboxes under the home

**Why:** agents' worktrees leave the CEO's folders for `~/.foreman/sandboxes/<project>/<agent>/` ([Projects anywhere](./epics/projects-anywhere.md)).

**Done when:**
- Worktrees are made at `~/.foreman/sandboxes/<project>/<agent>/<repo name>`, from repos anywhere; the project repo is mounted at `<sandbox>/project`, as today. No `.git/info/exclude` line any more (the sandbox isn't inside any repo).
- The mounts follow: the sandbox, each repo's `.git` read-only at its own path, the project repo; nothing else.
- The session prompt names the sandbox and its folders, never a project folder (`({path})` goes): the [container writes](./container-writes-outside-sandbox.md) card's invitation is gone with it.
- A repo outside `$HOME` is refused when added ([Projects as records](./projects-as-records.md)): on macOS that is what the Podman machine shares, so a mount never fails at start.
- Sandboxes in the old places (`<project>/.sandboxes/`) are left alone: a fresh start, nothing imported.
- Tests: the paths, the mounts and nothing else, the prompt.

- [ ] paths and mounts; the workflow tools' paths
- [ ] the prompt
- [ ] specs: Sandboxes and git policy, Session runner, Session prompt
