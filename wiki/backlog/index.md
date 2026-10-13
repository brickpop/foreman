# Backlog

**Goal:** the design phase is complete: every product and architecture decision is in `product/`, except three runtime facts. Next is the [walking skeleton](./walking-skeleton.md), the first code, which settles those facts and is the base everything else builds on.

Cards live in this folder (one `type: task` file each); retired ones in `archive/`. This page doesn't list them; query them:

```sh
just wiki list --where type=task --where status!=done --prefix backlog/          # everything live
just wiki list --where type=task --where tags=decision --where status!=done      # decisions waiting on the CEO
just wiki list --where type=task --where status=todo --where priority=high       # what's next
just wiki list --where type=task --where status=in-progress                      # in flight
```

See `WORKFLOW.md` at the project root for the conventions.
