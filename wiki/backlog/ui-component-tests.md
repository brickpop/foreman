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
- the New project dialog sends `clone` only with Clone, and goes on to its repos once created;
- the repo picker: an Existing repo's name follows the picked folder until edited, a New empty repo's path is the picked folder plus its name, and the browser offers no `..` at the start folder (`parent` empty).

- [ ] the dependencies and the vitest environment; the tech stack entry
- [ ] the tests above
