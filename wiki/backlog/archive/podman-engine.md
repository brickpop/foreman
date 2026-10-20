---
type: task
status: done
priority: high
tags: [feature]
epic: terminal-multiplexer
---

# Podman execution engine

**Why:** real agents run in rootless containers ([Session runner](../../product/architecture/session-runner.md)).

Was blocked by [Experiment A](./prove-a-podman-engine.md), proven on macOS on 2026-10-14: its results are the starting point.

**Done when:** `runner/podman` implements `Runner` and `Process` like `runner/direct` (start with mounts, env, labels and size; attach; resize; stop; wait; list for re-attach), with the Podman service started when needed, and integration tests behind a build tag that the CEO runs on macOS and Fedora.

- [x] libpod REST client over the socket (`net/http`), attach as a hijacked stream, attached **before** the start so the first output is kept
- [x] mounts at the same path, `keep-id`, `:z` on SELinux; create and start one at a time; a stopped container left with the same name is removed, a running one refuses the start; exited containers are removed (their name is free again)
- [x] Podman service start (`podman system service` / `podman machine start`); `Version()` for Settings → Maintenance (the section itself: [Image builder](./image-builder.md))
- [x] `just test-podman` on the CEO's Mac (2026-10-14) (and Fedora): the integration tests (first output, input, exit code, size and resize, Ctrl-P Ctrl-Q, mounts rw/ro and file ownership, re-attach from a second engine, stop, 12 starts at once)
