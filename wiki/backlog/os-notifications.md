---
type: task
status: todo
priority: medium
tags: [feature, decision]
epic: workflow
---

# Notifications from the OS, not only the browser

**Why:** [Notifications](../product/features/notifications.md) come only from an open Foreman tab, as browser notifications. With no tab open, the CEO hears nothing; with one, they depend on the browser's permission. The CEO wants a choice (the CEO, 2026-10-22).

## Decided (the CEO, 2026-10-22)

- **One setting, None | Browser | OS (server)**, `notifications.via` = `none` · `browser` · `os`, default `browser` (today's behaviour). The per-event toggles stay and apply to either. In-app toasts always show.
- **OS notifications come from the server**, which runs on the CEO's machine, so they reach the CEO with no Foreman tab open. Clicking one opens its page (the review, else the session) in the default browser, at the URL Foreman prints (its token is exchanged for the cookie as at startup).
- **The app tells the server whether the CEO is there**: each tab reports when it becomes visible or hidden (the Page Visibility API), and keeps reporting while visible, so a tab closed or a machine asleep counts as away once the reports stop. While a tab is visible, OS notifications are skipped, as browser ones are: its toast says it already.

## Proposal (to confirm)

- **One source for what a notice says**: the notice text (`noticeFor`, in the UI today) moves to the server, which publishes each notice as an event; the UI shows it as a toast (and a browser notification with `browser`), the server as an OS notification with `os`. Otherwise the titles and bodies are written twice, in TypeScript and Go.
- **Linux: Foreman sends them itself**, over D-Bus to the desktop's notification service (`org.freedesktop.Notifications`, what `notify-send` uses), with a Go D-Bus library (`github.com/godbus/dbus`, pure Go): no program to install, a click comes back as a signal (it opens the page), and a newer notice from the same agent replaces the older.
- With `listen = "lan"` the server may not be the machine the CEO looks at: the setting says OS notifications show on the machine Foreman runs on.

## Open questions

- **macOS.** macOS delivers notifications only from an app (a bundle with an identifier), never from a bare binary like `bin/foreman`, and the API is Objective-C (cgo). The ways:
  1. `osascript`: built in, but notifications show as from Script Editor, and a click opens Script Editor, not Foreman.
  2. `terminal-notifier` (`brew install terminal-notifier`): its own app; a click opens the page.
  3. Foreman as an app, `Foreman.app` with the binary inside, posting them itself (cgo, the `UserNotifications` framework): its own name and icon, a click opens the page. It changes how Foreman is built and started on macOS, and whether a locally signed (`codesign -s -`) build may post them must be proved on the CEO's Mac first.

**Done when:** the questions are answered; the setting, the presence reports, the server's notices and both OS paths are built and tested (what each OS is sent, the click target, nothing sent with `none`, a toggle off, or a tab visible); [Notifications](../product/features/notifications.md) and [API → Events](../product/architecture/api.md#events) say how.
