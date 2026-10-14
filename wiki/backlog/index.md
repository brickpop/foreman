# Backlog

**Current epics, side by side:** [Prove](./epics/prove.md) (the CEO runs the experiments that settle the five runtime facts) and [Full UI on a demo client](./epics/full-ui-demo.md) (the whole UI, on demo data where the server isn't ready). Then [Terminal multiplexer](./epics/terminal-multiplexer.md).

The epics, in order:
0. [Prove](./epics/prove.md) and, alongside it, [Full UI on a demo client](./epics/full-ui-demo.md)
1. [Terminal multiplexer](./epics/terminal-multiplexer.md)
2. [Messaging and hand-offs](./epics/messaging-handoffs.md)
3. [Git](./epics/git.md)
4. [Workflow with the CEO](./epics/workflow.md)
5. [Chief of Staff](./epics/chief-of-staff.md)

Cards live in this folder (one `type: task` file each, linked to its epic); retired ones in `archive/`. This page doesn't list them; query them:

```sh
just wiki backlinks /backlog/epics/prove.md | grep -E "^/backlog/[^/]+\.md:" | grep -v "^/backlog/index\.md:" | cut -d: -f1 | sort -u   # the cards of an epic
just wiki list --where type=task --where status!=done --prefix backlog/          # everything live
just wiki list --where type=task --where tags=decision --where status!=done      # decisions waiting on the CEO
just wiki list --where type=task --where status=todo --where priority=high       # what's next
just wiki list --where type=task --where status=in-progress                      # in flight
```

See `WORKFLOW.md` at the project root for the conventions.
