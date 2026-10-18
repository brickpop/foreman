---
type: task
status: backlog
priority: low
tags: [feature, decision]
epic: chief-of-staff
---

# Project config with the Chief of Staff

**Why:** `project.toml` changes through the Chief of Staff (journey step 27, use case 10) were taken out of the first CoS build (the CEO, 2026-10-18): the file lives in the project repo, which the lead also writes, so it can't simply follow the team files' "Foreman writes on the CEO's click".

**To decide:** a proposal confirmed in the drawer and committed straight on the project repo's `main` (like the lead's own commits; a commit could sweep in a lead's unfinished edit, as between leads today: [backlog](./index.md#not-yet-scheduled)), or the earlier proposal (a branch `foreman/config-N` and a review request on the project's Overview, which brings back merge requests on the project repo and a `cos` request source). Then: Project settings' `Change with Chief of Staff`, the [Chief of Staff](../product/features/chief-of-staff.md) (which today handles teams only), journey step 27 and use case 10.
