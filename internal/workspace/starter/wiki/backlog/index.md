# Backlog

**Current goal:** none yet: the lead writes it here.

Tasks live in this folder, one file each, grouped in epics (`epics/`). The Board shows the scheduled ones, the Backlog board the rest. Find them with:

- What's next: `wiki --root project/wiki list --where type=task --where status=todo --where priority=quick`, then `priority=high`
- In progress: `wiki --root project/wiki list --where type=task --where status=in-progress`
- Blocked: `wiki --root project/wiki list --where type=task --where status=blocked`
- Unscheduled: `wiki --root project/wiki list --where type=task --where status=backlog`
