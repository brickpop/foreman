# Backlog

**Current epics, side by side:** [Prove](./epics/prove.md) (the CEO runs the experiments that settle the five runtime facts) and [Full UI on a demo client](./epics/full-ui-demo.md) (the whole UI, on demo data where the server isn't ready). Then [Terminal multiplexer](./epics/terminal-multiplexer.md).

The epics, in order:
0. [Prove](./epics/prove.md) and, alongside it, [Full UI on a demo client](./epics/full-ui-demo.md)
1. [Terminal multiplexer](./epics/terminal-multiplexer.md)
2. [Messaging and hand-offs](./epics/messaging-handoffs.md)
3. [Git](./epics/git.md)
4. [Workflow with the CEO](./epics/workflow.md)
5. [Chief of Staff](./epics/chief-of-staff.md)

Cards live in this folder (one `type: task` file each, naming its epic in its `epic:` field); retired ones in `archive/`. This page doesn't list them: browse the **Epics** and **Backlog** boards, or query them:

```sh
wiki --root wiki list --where epic=prove                                               # the cards of an epic
wiki --root wiki list --where type=task --where status!=done --prefix backlog/          # everything live
wiki --root wiki list --where type=task --where tags=decision --where status!=done      # decisions waiting on the CEO
wiki --root wiki list --where type=task --where status=todo --where priority=high       # what's next
wiki --root wiki list --where type=task --where status=in-progress                      # in flight
```

See `WORKFLOW.md` at the project root for the conventions.

## Not yet scheduled

Raised during design, no cards yet; they get one when an epic needs them.

- **What agent "skills" mean technically**: plain text in the session prompt, or the harness's own skills feature; it decides what the Agent page's skill chips actually change.
- **Resource lifecycle**: cleanup of logs, images, agent memory and stopped deployments, and what happens when the laptop sleeps or the Podman VM wakes; without it, disk use grows forever and agents may stall after a sleep.
- **CI pipeline and how Foreman is installed** (release binaries, Homebrew): tests on every push, and a setup that doesn't require building from source.
- **History and debugging views** (past requests, past messages): when an agent misbehaves, the CEO needs to look back at what happened.
