---
type: task
status: done
priority: high
tags: [bug]
epic: terminal-multiplexer
---

# Lines vanish from the middle when the terminal shrinks

**Found (the CEO, 2026-11-05):** opening Line cut lines out of the middle of Claude Code's transcript, scrollback included, unnoticed until reading it. xterm.js deletes the rows below the cursor first, and Claude Code redraws its input box over the lines above it. Decided: shrink from the top ([Sessions → Body](../../product/features/sessions.md); [Technical debt](../../technical-debt.md#resizing-the-terminal-from-the-bottom)); keep Line under the terminal (a floating Line: not now); keep each agent's unsent Line text across switches, until a reload, which asks first.

- [x] `terminalFit.ts`: scroll the screen up before a shrink, output held meanwhile; every fit goes through it
- [x] the grow brings the lines back (the shrink alone doubled lines at each Line toggle, the CEO, 2026-11-05), and a filled screen's blank last row goes up too (keeping it drifted a line per toggle); both checked against a recorded Claude Code session
- [x] Line text kept per agent (`app/unsentLines.ts`), Line reopening with it; the agent keys still select in a Line holding text, never switch (the CEO, 2026-11-05: tried switching, wanted the selection back)
- [x] the browser asks before a reload while any running agent has a kept line
