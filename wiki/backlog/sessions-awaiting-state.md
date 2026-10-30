---
type: task
status: todo
priority: low
tags: [feature, ui, decision]
epic: terminal-multiplexer
---

# Sessions list: a deployment waiting on the CEO doesn't say `idle`

**Why:** in the [Sessions list](../product/features/sessions.md#the-list), the state word is `working` when an agent is running or starting, otherwise `idle`, `asleep` or `stopped` (`sessionRows` in `ui/src/app/sessions.ts`). A deployment whose only live agent is `awaiting` (stopped at a question to the CEO in its session) therefore reads `idle`, although it is blocked on the CEO. Only the status dot's colour and, when there is an open request, the amber needs-you dot hint at it. Raised 2026-10-29 while building the list (the CEO: a task for later).

**The same disagreement for `queued` and `done` (2026-10-30):** `working` covers `running` and `starting` only, so an agent `queued` for a session slot, or `done`, reads `idle` beside a dot that ranks it above `idle` (`liveliness` in the same file). Whatever words are chosen, they should settle those two as well as `awaiting`.

## To decide with the CEO

- The word: `waiting on you`, `needs you`, or something else.
- Does it beat `working` when one agent is awaiting and another is running? The status dot already ranks `awaiting` first (`deploymentStatus`).
- Do `queued` and `done` get words of their own (`queued`, `done`), or stay `idle`? The dot already draws `queued` dashed, so the row can contradict itself today.
- Is a separate needs-you dot still useful beside such a word? It comes from an open [request](../product/features/requests.md), and an `awaiting` agent may have none.

**Done when:** the row's word says the deployment waits on the CEO, with tests in `ui/src/app/sessions.test.ts` (the `says working, idle, asleep or stopped` case today pins `awaiting` to `idle`), and [Sessions → The list](../product/features/sessions.md#the-list) says so.
