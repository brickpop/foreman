---
type: task
status: todo
priority: high
tags: [spike]
epic: prove
---

# Experiment B: one Claude Code login shared by many containers

**Question:** can 12 Claude Code containers share one login file, read-write, and survive token refreshes, each picking up the others' refreshed token instead of using a revoked one? And how must each harness be pointed at the shared file so that its own writes land there? See [Session runner → Credentials](../product/architecture/session-runner.md#credentials), [Building blocks → What is proven](../product/architecture/building-blocks.md#what-is-proven-and-what-isnt).

Claude writes the recipe; the CEO runs it on macOS and Fedora.

**Procedure** (`just prove-b`):
1. Build a minimal image with Claude Code (the future harness layer).
2. Login container: run `claude /login` in a container with `.foreman/credentials/claude-code/` mounted read-write; the CEO completes the sign-in.
3. Try each way of exposing the shared file inside an instance's private `CLAUDE_CONFIG_DIR`: (a) the whole credentials folder as the config dir's credentials location, (b) a bind mount of the single file, (c) a symlink to the mounted folder. Record which survives Claude Code's own writes (an atomic rename replaces a symlink and fails on a single-file mount).
4. With the working variant, start 12 containers, each running a short non-interactive prompt (`claude -p "say ok"`) in a loop every few minutes, for long enough to cross at least two token expiries (the file's `expiresAt` says when; the recipe can shorten the wait by forcing an expired access token in a copy).
5. Log every failure and every change of the shared file.

**PASS:** no instance fails authentication across the refreshes, and the shared file always holds the latest token. **FAIL:** an instance fails after another refreshed.

**On FAIL:** the fallbacks in [Session runner → Credentials](../product/architecture/session-runner.md#credentials) (a keeper that refreshes for everyone, or one login per instance) become the design; the CEO picks one.
