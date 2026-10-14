---
type: task
status: done
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
4. With the working variant, start 12 containers, each running a short prompt (print mode is fine for this experiment; the product itself never uses it), in a loop every few minutes, for long enough to cross at least two token expiries (the file's `expiresAt` says when; the recipe can shorten the wait by forcing an expired access token in a copy).
5. Log every failure and every change of the shared file.

**PASS:** no instance fails authentication across the refreshes, and the shared file always holds the latest token. **FAIL:** an instance fails after another refreshed.

**On FAIL:** the fallbacks in [Session runner → Credentials](../product/architecture/session-runner.md#credentials) (a keeper that refreshes for everyone, or one login per instance) become the design; the CEO picks one.

## Progress

- **Login step (2026-10-14, macOS): PASS.** `just prove-b-login` connected the CEO's terminal to Claude Code in a container through Foreman's own attach code (raw terminal, both ways, resized to the terminal). The login (`.credentials.json`) landed in the shared credentials folder, mounted as the container's `CLAUDE_CONFIG_DIR`, separate from the Mac's own login.
- **Refresh race, first runs (2026-10-14, macOS, 12 at once):** many container *starts* failed for a Podman reason (concurrent `keep-id` starts race; now serialized, see [Session runner](../product/architecture/session-runner.md)). Counting only the containers that started:
  - **`dir`** (the whole shared folder as `CLAUDE_CONFIG_DIR`): **survived**. Round 1 with the token expired: 8/8 succeeded and the shared file got the refreshed token; round 2: 6/6. Claude Code evidently coordinates refreshes through something kept in its config folder (a lock), so one refreshes and the others read the new token.
  - **`symlink`**: **broke sharing**. One container refreshed and replaced the link with its own file; every other one failed with "OAuth session expired and could not be refreshed", and the shared file kept the expired token. The file afterwards held `expiresAt: 0` and a top-level `trustedDeviceToken`. An interactive start then *opened* without asking to log in, but that proves nothing: Claude Code's interface opens without contacting Anthropic, and the next real request failed. Whether `trustedDeviceToken` can recover a login is unknown.
  - **`file`** (the file alone in each private folder: the current design): **inconclusive** (2026-10-14). The run started from a login already unusable in print mode (`expiresAt: 0`, left by the symlink run; the first container to refresh failed), and Podman lost 10 creates (`EOF`) plus 5 starts even with starts serialized. To re-run from a fresh login: the race now starts with a baseline round (one container) and stops if the login doesn't work.
  - **`file`, re-run from a fresh login (2026-10-14, macOS, 12 at once): PASS.** Baseline 1/1. Round 1 with the token marked expired: 12/12, and the shared file got the refreshed token (so Claude Code writes it in place: a rename onto a single-file mount would have failed). Round 2: 12/12. Creating and starting one container at a time, Podman lost none. Sharing the login file alone, in otherwise private config folders, survives the refresh race in print mode. Still open: the same in interactive sessions, and a refresh that happens mid-session rather than at start.
  - **Interactive vs print mode:** the product runs interactive sessions only, so the race will also be run interactively, with the session driver experiment D needs.
  - **Start storm:** concurrent create + start with `keep-id` breaks Podman starts ([Session runner](../product/architecture/session-runner.md)); the race now creates and starts one container at a time, and the login step forces a real login and checks it with one request.
- **Claude Code's login lives in two places** (2026-10-14): the credentials (`.credentials.json`) and an account record in `.claude.json`. With the credentials moved aside, Claude still opened as if logged in and never asked; only the `/login` command runs the real login flow. The harness adapter must handle both files (which to share, which to keep private), and the Connect flow in Settings must run `/login` explicitly.
