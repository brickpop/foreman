---
type: task
status: todo
priority: high
tags: [chore]
epic: install-first-run
---

# Remove the forge

**Why:** the CEO, 2026-11-15: "I prefer having clean slates to work with, and I see no immediate need for this... not to mention that I prefer to keep commits local and then push them signed here." Foreman never talks to GitHub, Forgejo or Gitea: merges are local, the CEO pushes code with their own git, and the HQs reach their remote through each repo's own `origin` ([Portable projects](./portable-projects.md)). Done first in the folder work, since it shrinks everything [One Foreman folder](./foreman-folder.md) and Portable projects touch.

Little is lost: no adapter was built (`internal/forge` holds only the `Account` wire type), and the design stays in git history. **Reading a PR's comments**, if wanted later, is a fresh card: the agent's branch → its PR, its comments fetched through the CEO's own forge CLI login (`gh`), no token stored by Foreman (listed in the backlog's Not yet scheduled).

**What goes:**

- `project.toml`'s `[forge]` table (`kind`, `url`, `project_repo`): the starter loses it; a file still carrying it gets the usual unknown-key problem, no special case (the CEO's files are cleaned by the move script, [One Foreman folder](./foreman-folder.md))
- the global settings' `forges` table and `global_forge`; Settings → Forge accounts; Project settings' forge block and **Sync** buttons (sync becomes automatic: Portable projects)
- the **Secrets** spec, whose only content was forge tokens (the store was never built), and Security's lines on forge tokens
- `internal/forge`, the `Account` type and its generated UI type (`tygo.yaml`), the UI's `forgeOf` and every Create PR button, PR link and forge CI status on merge requests and reviews
- the [The forge](./git-forge.md) card (retired with this one's first commit), the Git and Workflow epics' forge PR parts, and the forge lines in [Review page](./review-page.md) and [After a merge](./after-merge-cleanup.md)

**Kept:** signing the CEO's commits from Foreman's process ([Technical debt → Signing](../technical-debt.md#signing-as-the-ceo-from-foremans-process)): the CEO pushes those commits, signed, themselves. The debt item's "Why" now names the CEO's own push, not a forge.

## Steps

- [ ] code: `internal/forge`, settings' `forges`/`global_forge`, `project.toml`'s `[forge]`, the merge request's PR fields (`ForgePRURL`, `ForgePRNumber`) (a store migration drops their columns)
- [ ] UI: Forge accounts, the forge block and Sync in Project settings, Create PR, PR links, forge CI statuses, `forgeOf`
- [ ] specs: forge-adapters and secrets deleted; project-config, global-settings, settings, project-settings, pipeline-and-merges, reviews, review, merge-request, work-item, deployment, api, protocol, security, sandboxes-and-git, session-prompt, session-runner, overview, building-blocks, tech-stack, home, add-project, glossary, vision, journey, use-cases, design-system, index; the `forge` spec tag leaves WORKFLOW.md
- [ ] backlog: [The forge](./git-forge.md) retired; Git and Workflow epics' exit criteria without a forge PR; Review page, After a merge, Polish a project, No Co-Authored-By lose their forge lines
- [ ] tests: the forge's tests go; a merge request round-trips without PR fields; `grep -ri forge` over code, UI and specs finds nothing but the archive
