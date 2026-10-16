---
type: epic
status: done
order: 0
---

# Prove

**Goal:** settle the five runtime facts the architecture rests on, before anything is built on them ([Building blocks → What is proven](../../product/architecture/building-blocks.md#what-is-proven-and-what-isnt)). Each one is a small, standalone experiment with a yes/no outcome.

**This epic gates epics 1 and 2:** a FAIL changes the design (the specs first, then the cards) before the blocks that depend on it are built.

**How it runs:** Claude writes each experiment as a `just` recipe that prints PASS or FAIL with what it observed; the CEO runs it on macOS (`podman machine`) and Fedora (rootless Podman, SELinux) and sends the output back. The outcome is written into the specs it concerns, then the card is deleted.

**Usable when done:** nothing new for the CEO: a design that is known to work.

**Journey steps it underpins:** 1–3 and 9 (engine, images, login), 13–14 (agent link, delivery), 21–22 (waiting for the CEO). See [the journey](../../product/journey.md).

**Exit criteria:** every experiment card is retired with PASS, or with FAIL and the specs already changed to the alternative design.

**Cleanup (ephemeral code):** `cmd/prove/`, `cmd/proveagent/` and the `prove-*` recipes are throwaway, committed only so the experiments can run. Remove them once what they prove has landed in real code: the Podman client in the [Podman engine](../podman-engine.md), the image build in the [Image builder](../image-builder.md), the login in the [Claude Code harness](../claude-harness-and-login.md), and experiment D run on the real engine. Removed on 2026-10-16: experiment D's checks became `just test-claude` ([Claude Code integration tests](../claude-integration-tests.md)).

**Done (2026-10-14):** A, B, C, D and E all passed for Claude Code on macOS (Fedora not run; macOS is the CEO's machine). The one design consequence: multi-line deliveries into Claude Code are not pasted ([D](../prove-d-claude-delivery-hooks.md)). Trimmed (2026-10-15): experiments A, C, the storm check and B's race are gone (code and recipes); what stays is what experiments D and E need (`prove-image`, `prove-b-login`, `prove-d`, `prove-e`, `cmd/proveagent`), removed when their checks become the Claude Code adapter's integration tests in [Messaging and hand-offs](./messaging-handoffs.md).
