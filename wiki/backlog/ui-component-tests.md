---
type: task
status: todo
priority: low
tags: [test]
epic: terminal-multiplexer
---

# UI component tests

**Why:** the CEO, 2026-10-17 ("yes, but later"). The UI's tests (vitest) cover pure functions only: nothing renders a component and clicks it, so what a screen does (where a card leads, what a dialog sends) is checked by hand. Found while building [Team → Deployments: a card opens its sessions](./archive/team-deployment-cards.md), whose card asked for such a test.

**Done when:** `@testing-library/react` and `happy-dom` are dev dependencies (Bun), vitest runs component tests under `just test`, and the first ones exist:
- the Team page's deployment card opens the project's Session, and its project name opens the project's Wiki;
- the New project dialog sends `clone` only with Clone, and goes on to its repos once created;
- Project settings: the Folders' Copy; Remove sends `deleteWorktrees` and `deleteHQ` only when ticked, both unticked at each opening;
- Deployment's Clean up: the uncommitted option shows only when a worktree holds uncommitted work, is unticked at each opening, and is sent as ticked;
- Settings → Agents → Instructions: Save and Discard only once edited;
- Settings → Instructions for every agent: Save and Discard appear only once the text differs; a failed save keeps the text; once saved, the field shows it (no flash of the old text) and follows a change on disk while not edited;
- Deploy a team: an archived project reads `<id> (archived)`, Deploy stays disabled while it is chosen, and the row's Unarchive sends `setArchived(id, false)` then enables Deploy;
- the repo picker (`AddRepo`): a repo folder's name follows it until edited, a plain folder's new repo is that folder plus the name typed, a name typed for one kind of folder is dropped on the other; a pasted path refused leaves the browser where it was.

- [ ] the dependencies and the vitest environment; the tech stack entry
- [ ] the tests above
