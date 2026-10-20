---
type: task
status: todo
priority: high
tags: [bug]
---

# Writes outside the sandbox are silently lost

**Why:** seen 2026-10-18 on Gravity. Arca (a member on Pi) reported that its write and edit tools "report success but nothing appears on disk". Pi's write tool reports success only after the write lands (`dist/core/tools/write.js`, 1.3.0), so the file went somewhere other than where Arca looked. The logs from that session are gone, so the path isn't confirmed.

**The hole, whatever Arca did:** a container mounts only the agent's sandbox, the repos' `.git` folders and `project/` inside the sandbox ([Sandboxes and git policy](../product/architecture/sandboxes-and-git.md)). The rest of the container's filesystem is its own writable layer. A write anywhere else succeeds, reaches nobody (not the CEO's disk, not a teammate) and is gone at the next restart. Every harness has this hole, not only Pi. The [session prompt](../product/architecture/session-prompt.md) invites it: `Project: {project}  ({path})` names the project's full path, and under it only `.sandboxes/<agent>` is mounted. So `<path>/project/wiki/x.md` (the live wiki, not through the sandbox) is such a write.

**Options (the CEO decides):**
- **A read-only root filesystem** (`podman run --read-only`, keeping tmpfs for `/tmp`): a write outside the mounts fails loudly, as `project/` already does for members. It needs a check of what the harnesses and toolchains write under `$HOME` (caches, `~/.npm`), which may need a tmpfs or a mount of their own.
- **The prompt only:** drop `({path})`, or say that only the sandbox is kept. That's cheaper, but nothing enforces it.

**Changed by [Projects as records](./archive/projects-as-records.md) (2026-10-20):** the prompt now names only the sandbox, so the invitation is gone; a write outside the mounts still lands in the container's own layer, so the read-only root stays an option.

- [ ] the CEO's choice
- [ ] the fix, with a test that a write outside the sandbox fails (or isn't invited)
- [ ] the specs: Sandboxes and git policy, Session prompt
