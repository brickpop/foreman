---
type: task
status: in-progress
priority: medium
tags: [feature]
epic: install-first-run
---

# Pi as a second harness

**Why:** the CEO, 2026-10-18: a second [harness](../product/architecture/harness-adapters.md) besides Claude Code, for agents on models the CEO serves locally or on other providers. Brought forward ahead of [Idle sleep](./archive/idle-sleep.md) the same day.

**Decided (the CEO, 2026-10-18):**
- **A Pi profile is a model server and one model**, Pi's `models.json` in the profile folder, set in a form: URL, optional API key, Detect (`<url>/models`) to pick the model, context window and longest reply prefilled with 256k and 128k. `samplingParams`, `thinkingLevelMap` and the like are optional and added by hand; a save keeps them. One model per profile: another model is another profile, until [presets](./agent-model-settings.md).
- **`localhost` is rewritten to the host alias**, only for containers (Podman), never on the Direct engine or for another host.
- Pi's own `/login` (subscriptions) isn't offered. Using a Claude subscription through Pi may not be allowed by Anthropic's terms anyway.

**Probed** on Pi 1.3.0 in a terminal against a stand-in OpenAI-compatible server (2026-10-18): no dialog at startup with `--no-approve`; the opener as the message argument, with `--continue` too, and `--continue` with nothing saved starts fresh; a typed line then Enter after a pause is one message; Ctrl+J is a new line; `--provider` and `--model` take ids with `:` and `/` exactly; MCP tools reach the model only with `"exposure": "direct"`; the hooks extension relays its events in order, and a slash command starts no run. The adapter's own generated launch ran real Pi end to end. What was built is in [Harness adapters → Pi](../product/architecture/harness-adapters.md#pi).

- [x] the probe, its results in [Harness adapters → Pi](../product/architecture/harness-adapters.md#pi)
- [x] `internal/harness/pi`: Launch, Interpret, Describe, Connected, Newline, the hooks extension (`hooks.ts`, embedded), the profile's `models.json` (Endpoint, SetEndpoint, Detect); `harness.EndpointAdapter`, `harness.Conversations` and `harness.GitConfig` shared with Claude Code's adapter
- [x] the routes (`GET`/`PUT /api/profiles/:name/endpoint`, `POST …/endpoint/models`), `connect` in `GET /api/harnesses`, the Connect dialog's form
- [x] Pi in the agent image (after Claude Code), the adapter in `main.go` with the host alias
- [x] tests: the launch (config dir, models copy, host alias, prompt, MCP, extension, continue and opener), hand edits kept, detection, the routes, the wiring, the client
- [x] `ask_user` in the extension: a dialog in Pi's terminal, read as awaiting while open (the CEO, 2026-10-18; probed on Pi 1.3.0)
- [x] a project's change of profile: running agents still on another profile listed in Project settings, with Restart them (`POST /api/instances/:id/restart`); another harness starts fresh, a lead with its goal; the instance keeps its profile (migration 010)
- [ ] **the CEO's check on the Mac:** a Pi profile on a real server, a deploy, the team tools called, the activity lines, a restart that continues
- [ ] a real-Pi integration test like `just test-claude` (needs a model server: the CEO's, or a stand-in)
- [ ] Linux: a model on the host reached from Podman (the same question as the agent port, [First run](./first-run.md))
