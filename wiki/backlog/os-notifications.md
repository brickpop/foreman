---
type: task
status: todo
priority: medium
tags: [feature]
epic: workflow
---

# Notifications from the OS, not only the browser

**Why:** [Notifications](../product/features/notifications.md) come only from an open Foreman tab, as browser notifications. With no tab open, the CEO hears nothing; with one, they depend on the browser's permission. The CEO wants a choice (the CEO, 2026-10-22).

## Decided (the CEO, 2026-10-22)

- **One setting, None | Browser | OS (server)**, `notifications.via` = `none` · `browser` · `os`, default `browser` (today's behaviour). The per-event toggles stay and apply to either. In-app toasts always show.
- **OS notifications come from the server**, which runs on the CEO's machine, so they reach the CEO with no Foreman tab open. Clicking one opens its page (the review, else the session) in the default browser, at the URL Foreman prints (its token is exchanged for the cookie as at startup).
- **The app tells the server whether the CEO is there**: each tab reports when it becomes visible or hidden (the Page Visibility API), and keeps reporting while visible, so a tab closed or a machine asleep counts as away once the reports stop. While a tab is visible, OS notifications are skipped, as browser ones are: its toast says it already.
- **One source for what a notice says**: the notice text (`noticeFor`, in the UI today) moves to the server, which publishes each notice as an event; the UI shows it as a toast (and a browser notification with `browser`), the server as an OS notification with `os`. Otherwise the titles and bodies are written twice, in TypeScript and Go.
- **Linux: Foreman sends them itself**, over D-Bus to the desktop's notification service (`org.freedesktop.Notifications`, what `notify-send` uses), with a Go D-Bus library (`github.com/godbus/dbus`, pure Go): no program to install, a click comes back as a signal (it opens the page), and a newer notice from the same agent replaces the older.
- **macOS: `terminal-notifier`** (`brew install terminal-notifier`): macOS delivers notifications only from an app, never from a bare binary like `bin/foreman`; `terminal-notifier` is one, and a click opens the page. Without it Settings says to install it, and nothing is sent (`osascript` was turned down: a click opens Script Editor). Foreman posting them itself waits for the [desktop app](./desktop-app.md).
- **Behind an interface** (`Notifier`): the desktop app swaps in its own sender without touching anything else.

**Done when:** the setting, the presence reports, the server's notices and both OS paths are built and tested (what each OS is sent, the click target, nothing sent with `none`, a toggle off, or a tab visible); [Notifications](../product/features/notifications.md) and [API → Events](../product/architecture/api.md#events) say how.
