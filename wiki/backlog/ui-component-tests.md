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
- the Team page's deployment card opens that deployment's Sessions, and its project name opens the project;
- the New project dialog sends `clone` only with Clone, and goes on to its repos once created; its workspace follows the name (`<projects folder>/<name>`) until edited, and says when the projects folder will be created;
- Project settings: the workspace's Copy; for a project made before workspaces, the "Move into a workspace" offer pre-filled and sent; Remove sends `deleteWorktrees` and `deleteRepo` only when ticked, both unticked at each opening;
- Settings → System → Projects folder: Save and Discard only once edited, as the instructions;
- Settings → Instructions for every agent: Save and Discard appear only once the text differs; a failed save keeps the text; once saved, the field shows it (no flash of the old text) and follows a change on disk while not edited;
- the repo picker: an Existing repo's name follows the picked folder until edited, a New empty repo's path is the picked folder plus its name, and the browser offers no `..` at `$HOME` (`parent` empty).

- [ ] the dependencies and the vitest environment; the tech stack entry
- [ ] the tests above
