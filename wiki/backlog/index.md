# Backlog

**Goal (design phase):** finish the full design of Foreman (product definition, architecture and stack) and resolve every open decision before writing any code. The phase ends when no `decision` card is open and the spikes have confirmed the risky parts of the design.

Cards live in this folder (one `type: task` file each); retired ones in `archive/`. This page doesn't list them; query them:

```sh
just wiki list --where type=task --where status!=done --prefix backlog/          # everything live
just wiki list --where type=task --where tags=decision --where status!=done      # decisions waiting on the CEO
just wiki list --where type=task --where status=todo --where priority=high       # what's next
just wiki list --where type=task --where status=in-progress                      # in flight
```

See `WORKFLOW.md` at the project root for the conventions.
