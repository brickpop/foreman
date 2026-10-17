---
type: task
status: todo
priority: high
tags: [bug]
epic: terminal-multiplexer
---

# A woken agent's pane stays black until a key, and that key goes to Claude Code

**Seen by the CEO (2026-10-16):** when an agent is woken, its pane sometimes stays black. It shows up only after a keypress, which Claude Code takes as input: the CEO lands in its agents list (an arrow key) and has to press left arrow, or similar, to get back.

**Hypotheses** (to check first):
- **The pane hides the replay** until output arrives (`Terminal.tsx`: drawn hidden, shown at once, with a 600 ms fallback). On a fresh start the first connect may arrive before the session has a terminal, or the view may stay hidden.
- **Size:** the session starts at the default 120×40 and gets the pane's size only on the first resize. Claude Code's fullscreen-free UI may wait for a redraw that only input triggers.
- **The pane re-mounting** when the status changes from asleep to starting to running may connect twice, or to a terminal that's about to be replaced.

**Done when:** a woken agent's pane shows its screen without a keypress, every time; a test covers the start sequence (asleep, starting, running) with the pane open.

- [ ] reproduce (fake harness first; then Claude Code), find which hypothesis
- [ ] fix, with a test
