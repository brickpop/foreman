---
type: task
status: todo
priority: low
tags: [feature]
epic: terminal-multiplexer
---

# Drop or paste images into the UI for agents to read

**Why (the CEO, 2026-10-17, a future feature):** the CEO wants to show an agent a screenshot or a mockup. Today an image can't cross the browser: a pasted or dropped image isn't sent to the agent's terminal, and agents only see the folders mounted into their container ([Sandboxes and git policy](../product/architecture/sandboxes-and-git.md)). Foreman should take the image, store it in the home or under the project, make that path readable inside the agents' containers, and hand over a path the agent can open (Claude Code reads an image from a path written in its prompt).

**To decide when the card starts:**
- **Where uploads live.** Under the project, every agent of the project could read them through a mount. Inside `project/` they are already mounted, but they would be files in the project repo: committed, or git-ignored. A folder elsewhere in the home isn't mounted in any container today, so it needs a new mount, read-only.
- **Where the CEO drops or pastes:** the Sessions pane (terminal and [Line](../product/features/sessions.md)), the Wiki reader, anywhere.
- **What the agent receives:** the path typed into the Line or the terminal at the cursor, or a message.
- **The path the agent sees** is the container's view, not the host's: the two differ.
- **Limits and safety:** accepted types (images only, checked by content, not by name), a maximum size, file names chosen by Foreman (never the browser's, so no path traversal and no collisions), and whether uploads are ever cleaned up.

**Done when:** a dropped or pasted image ends up where the project's agents can read it, and the agent gets a path that opens it in its container; tests cover the limits and the naming.

- [ ] decide the points above with the CEO
- [ ] spec (Sessions, the API, Sandboxes and git policy for the mount), the upload endpoint, the UI, tests
