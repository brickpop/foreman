---
type: task
status: todo
priority: low
tags: [feature, decision]
epic: install-first-run
---

# Foreman as a desktop app

**Why:** on macOS only an app can post notifications of its own ([OS notifications](./os-notifications.md) use `terminal-notifier` meanwhile), and an app gives Foreman its name and icon, its own windows (a session per window), and a click on a notification that brings Foreman forward (the CEO, 2026-10-22: track it for later).

## The idea

- A [Wails](https://wails.io) v3 app: Foreman's server runs inside it as now, and its windows show the UI at `http://localhost:<port>`; the UI doesn't change, and a browser keeps working beside it.
- Its notifications service is a second `Notifier`, swapped in for `terminal-notifier` on macOS.
- `bin/foreman` stays, headless (on this machine only: there is no local-network mode, the CEO, 2026-10-30).

## Before starting

- **Wails v3 final**: at 2026-10-22 the latest tag is `v3.0.0-beta.8` (API called stable, issues may remain). v2 is stable but has no notifications and one window.
- **cgo**: the webview needs it, so each OS builds on that OS, and the release build (both CPU types in one binary) changes; Linux needs WebKitGTK at run time.
- **Distribution on macOS**: signing and notarizing (an Apple Developer account) to hand it to others; a locally signed build is enough on the CEO's Mac, to be proved.
- **Windows is out of scope**: Wails builds for it, but Foreman doesn't run there (Podman, git worktrees and the terminals assume macOS or Linux).

**Done when:** decided once v3 is final; then `Foreman.app` (and a Linux build) starts the server, opens the UI in its window, and posts notifications itself.
