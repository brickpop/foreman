---
type: spec
status: planned
tags: [feature, architecture]
---

# The end-to-end journey

One continuous story through everything the UI reference allows, from installing Foreman to a merged PR and beyond. Each step says what the CEO does and sees, then **under the hood**: which [building blocks](./architecture/building-blocks.md) act, through which interface. A step no block can explain is a gap; gaps are collected at the end. The [use cases](./use-cases.md) are the acceptance checks for the same ground.

Cast: the CEO; the workspace `~/agents-ws`; project `ledger` (repo `ledger-api`, already cloned there); team **Atlas** (Mara, lead; Tove and Ilya, engineers; Quill, scrum master); later team **Vesper** (a challenge team).

## Part 1: set up

**1. Start Foreman.** `foreman ~/agents-ws`. The first time it asks `Create .foreman/ here? [y/N]`; then it prints the URL with the access token and opens the browser.
- *Under the hood:* Workspace & config (opens or creates `.foreman/`, the access token); Web server (listens on 127.0.0.1:8888, access guard swaps the URL token for a cookie); Instance manager (re-attaches to agent containers still running from last time: none yet); Execution engine (Podman service started if needed: `podman system service` on Linux, `podman machine start` on macOS).

**2. Prepare the agent images.** Settings → Maintenance shows Base and Harnesses `not built yet`. The CEO presses **Refresh** (or the first deploy does it).
- *Under the hood:* Image builder (Podman): builds `localhost/foreman/base`, then `…/harness` (Claude Code, Pi…); workspace and project layers are transparent tags. Build logs stream to the page; the Event bus pushes the state changes.

**3. Log in to Claude Code.** Settings → Harnesses → Claude Code → **Connect**. A dialog shows a terminal running `claude /login`; the CEO completes the browser sign-in.
- *Under the hood:* a one-off **login container**: Execution engine starts it from the harness image with the shared login folder `.foreman/credentials/claude-code/` mounted read-write; a Terminal streams it into the dialog exactly like any session. When it exits, Settings shows `connected`.

**4. Connect the forge.** Settings → Forge accounts → GitHub → **Connect**: host and token, checked against GitHub, never shown again.
- *Under the hood:* Forge adapter (validates the token); the secret store (`.foreman/secrets`, 0600, never mounted into containers).

**5. Add the project.** Sidebar → **+ New project** lists the workspace folders that aren't projects yet, with their git repos. The CEO picks `ledger` → **Add project**.
- *Under the hood:* Workspace & config writes `ledger/.foreman/project.yaml` (repos found, `forge`), and a `wiki/` with `index.md` and `wiki.toml` (one Backlog board) if missing. The file watcher notices; the sidebar updates through the Event bus.

**6. Read the project.** Project → **Overview** shows the wiki's `index.md`; **Board** shows the Backlog board (empty); **Settings** shows `project.yaml` read-only.
- *Under the hood:* Wiki block (the agentic-wiki engine reads the bundle; markdown rendered on the server; links resolved by the engine).

**7. Create a team.** Sidebar → **+ New team** opens the Chief of Staff drawer: "What is the new team for?" → "Build features for ledger" → a proposed roster (lead, two engineers, scrum master) → **Create Atlas**.
- *Under the hood:* the Chief of Staff is an instance like any other (Instance manager, Harness adapter, Terminal, Execution engine) with **extra team tools** (`present_choices`, `show_panel`, `create_team`, `hire`…). The drawer renders its tool calls as chat; `create_team` is handled by Workflow, which writes `.foreman/teams/atlas.yaml` through Workspace & config. The file watcher picks it up and the sidebar shows Atlas. (Gap G2: how the Chief of Staff runs.)

**8. Tune an agent.** Team → Members → Tove → the Agent page: the CEO changes her look and personality. "Saved. Applies on every project."
- *Under the hood:* Workspace & config writes the team file (version-guarded against concurrent edits).

## Part 2: put the team to work

> **Phasing.** Until the [Git epic](../backlog/epics/git.md), every agent of a deployment works in the project folder itself (mounted read-write), with no clones, worktrees or per-agent branches. Steps 9.1 (the clones), 16 (the branch hand-off), 17 (integration) and the push in 18 rely on the Git epic; before it, a hand-off is a message ("done, see my changes").

**9. Deploy.** Team → **Deploy to a project** → `ledger`, goal "Add idempotency keys to POST /settle" → **Deploy**.
- *Under the hood:* the deployment is stored (State store). Instance manager `Ensure(deployment)`, for each member:
  1. Git: clone `ledger-api` into `ledger/.sandboxes/<agent>/ledger-api` (from Foreman's mirror; Mara's clone gets `lead/atlas/integration`).
  2. Image builder `Ensure(ledger)` → `localhost/foreman/ws/agents-ws/ledger`.
  3. Harness adapter `Launch`: the session prompt (soul + team + project + goal) as the instance's `CLAUDE.md`, the MCP config pointing at `foreman-agent`, the hooks config.
  4. Agent link: registers the instance's token.
  5. Execution engine `Start`: the container with its clone (rw), the wiki (rw for Quill, the scrum master, read-only for the others), its private config, the shared login file, `~/.gitconfig` (ro).
  6. Terminal: created on the process; status `starting` → `idle` on the first hook.
  7. **Kickoff:** the lead receives the goal as a first delivery, so it starts working. (Gap G1: not specified yet.)

**10. Watch.** Home: "4 sessions working across 1 deployment". **Sessions** → tab "Atlas · ledger" → Mara's pane (the lead's tab is first).
- *Under the hood:* Web server → Terminal `Attach` → a View: the snapshot of Mara's screen, then live output (WebSocket, raw bytes).

**11. Talk to the lead.** The CEO types into Mara's pane: "Use the existing request-id header; keep the API backwards compatible."
- *Under the hood:* keystrokes go raw, Web server → Terminal `Input` → the container's terminal. No Foreman logic in between.

**12. The lead plans in the wiki.** Mara reads `wiki/` and the code, and creates the card `wiki/backlog/LED-44.md` (a task: status `todo`, assignee `tove`). (Scrum masters own the wiki; a lead writes it when the team has no scrum master. Here Mara asks Quill, or the team's playbook lets her write cards: see Gap G3.)
- *Under the hood:* the agent edits files in its mounted `wiki/`; Foreman's file watcher notices; the Wiki block re-reads; the Event bus updates the Board and the "changed" dot in the reader.

**13. The lead delegates.** Mara calls the team tool `ask(tove, "Take LED-44 …")`.
- *Under the hood:* the harness → `foreman-agent mcp` (stdio) → the agent port → Agent link (the token says `mara@ledger`) → Messaging: message `req-41` queued for `tove@ledger`; `ask` returns `req-41` at once, Mara keeps working. A **marker** "Mara asked Tove (req-41)" appears beside both panes and in the Messages drawer (UI only).

**14. The engine wakes the teammate.** Tove is idle, so Messaging delivers: the text appears in Tove's terminal as if typed, and she starts.
- *Under the hood:* Instance manager knows Tove is `idle` (her last hook event); Messaging → `Instances.Deliver` → Harness adapter `Deliver` (types the text, then Enter) → Terminal `Input`. Tove's next hook says `busy`. Had she been busy, the message would wait for her next `idle`.

**15. The teammate works.** Tove reads `wiki/backlog/LED-44.md`, creates `wt/tove/LED-44` in her clone, writes the code and tests, runs `npm test` (the toolchain comes from the project's image layer), commits.
- *Under the hood:* nothing in Foreman: plain git and tools inside her container. The CEO can watch it live in her pane.

**16. Hand-off.** Tove calls `handoff(mara, "wt/tove/LED-44", "Idempotency keys, tests pass")`.
- *Under the hood:* Workflow: a work item (stage *handed to lead*). **Git** fetches `wt/tove/LED-44` from Tove's clone into Mara's (agents never push anywhere: containers can't reach each other, and nothing goes to the forge yet). Messaging delivers "Tove handed off wt/tove/LED-44: …" to Mara at her next idle; marker shown; Changes → In flight shows the item.

**17. Integration.** Mara reviews the branch (git diff in her clone) and merges it into `lead/atlas/integration`.
- *Under the hood:* plain git in Mara's container. Workflow notices the integration through Git (the branch is contained in a `lead/atlas/*` branch) when Mara next goes idle: the work item moves to *integrated*. (If she wanted changes instead: an `ask` back to Tove; see the open question in [Protocol](./architecture/protocol.md#open-questions).)

**18. The pull request.** Mara calls `open_pr("lead/atlas/integration", "Idempotency keys for /settle", summary)`.
- *Under the hood:* Workflow runs the project checks (in a throwaway container from the project image); Git fetches the branch from Mara's clone and pushes it to GitHub with the CEO's token; Forge adapter opens PR #128; Workflow creates the pull request entry and a **review request**. Home shows it, the browser notifies, Mara receives "PR #128 opened".

**19. The CEO merges.** Home → the review → **Open PR ↗** → the CEO reviews and merges on GitHub.
- *Under the hood:* Forge adapter polling sees `merged`; Workflow resolves the request, marks the PR and its work items merged, delivers "PR #128 merged" to Mara, and tells the leads of other open PRs on `ledger-api` to rebase. Git updates Foreman's mirror; clones fetch the new `main` at their next idle.

## Part 3: intervene

**20. A direct instruction.** In Sessions, the CEO presses **Split** and clicks Ilya's tab: his pane opens on the right with his current screen, banner "Direct line to Ilya. He'll tell Mara if it matters." The CEO types "Use the decimal library already in the repo."
- *Under the hood:* a second Terminal `Attach` (snapshot + live); keystrokes raw to Ilya's terminal. Ilya's prompt tells him to inform the lead if relevant, which he does with `ask`/`reply` like any message.

**21. An approval.** Ilya wants a new dependency, a project gate: he calls `escalate(approval, "Add decimal.js?", ["Approve", "Deny"])`. Home shows the request, the browser notifies. The CEO clicks **Approve**; Ilya continues.
- *Under the hood:* Agent link → Workflow: a request; **the tool call stays open**, so Ilya is paused; Event bus → Home and notification. `Resolve` returns "Approve" as the tool's result and Ilya resumes.

**22. The harness itself waits.** Pi stops at its own question ("Overwrite migrations? 1/2"). Home shows "Rhea is waiting in her session" with the choices; the CEO picks one, and it is typed into the session.
- *Under the hood:* the harness's hook → `foreman-agent hook` → Agent link → Instance manager; the Harness adapter's `Interpret` says *waiting for the user* (not just idle) → Workflow raises the request; the answer goes through `Deliver`.

**23. A crash.** Ilya's harness hits its rate limit and exits. Home: a **blocked** request, "Retry now · Retry in 30 min · Switch harness".
- *Under the hood:* Terminal `Done` → Instance manager → Harness adapter `Diagnose(exit code, last output)` → Workflow raises the blocked request; **Retry** makes the Instance manager start the instance again (same clone, same private config).

**24. A second team.** The CEO deploys **Vesper** (a challenge team) on `ledger` too. Kestrel finds a race and calls `escalate(finding, …, ["Atlas fixes it now", "Add to backlog", "Not an issue"])`. The CEO picks "Atlas fixes it now".
- *Under the hood:* two deployments on one project: separate instances, clones, lead branches (`lead/vesper/*`) and message channels (Messaging never crosses deployments). Workflow turns the answer into a delivery to Atlas's lead, Mara.

**25. Hire.** Team Atlas → **Hire** → the Chief of Staff interview → "Hire Arden". Arden appears in the team and starts on every Atlas deployment.
- *Under the hood:* the Chief of Staff's `hire` tool → Workflow → the team file; the file watcher → Instance manager `Ensure` starts the new instance (as in step 9).

**26. Edit a soul while running.** The CEO edits Tove's job description. Running instances get a short context-update delivery at their next idle; the full prompt applies at their next start.
- *Under the hood:* Workspace & config (team file) → Instance manager → Messaging delivery; the Harness adapter regenerates `CLAUDE.md` for the next start.

**27. Change project config safely.** Project → Settings → **Change with Chief of Staff** → "Send PRs to GitHub". A review appears in Changes.
- *Under the hood:* the Chief of Staff edits `project.yaml` on a branch `foreman/config-1` (in its own clone of the project repo) → Workflow: PR on the forge (or check-the-branch without one) → review request.

**28. Wiki and board.** In Overview, the CEO ticks a checkbox in a standup note; on the Board, drags LED-51 to "In progress".
- *Under the hood:* the Wiki block's version-guarded single-line writes (`SetCheckbox`, `SetField`); agents see the change in their mounted `wiki/`.

## Part 4: stop and resume

**29. Stop a deployment.** Team Atlas → Deployments → **Stop** on ledger, after confirming. Its sessions end; clones are kept; Vesper keeps running.
- *Under the hood:* Instance manager stops Atlas's instances (Execution engine `Stop`), revokes their tokens; its open requests expire; Messaging expires its queue.

**30. Restart Foreman.** The CEO quits and starts `foreman` again (an upgrade, say). Vesper's agents never stopped.
- *Under the hood:* the containers kept running; Instance manager `List`s them by label and re-attaches; their Terminals are rebuilt. (Gap G4: the server-side screen after a restart.)

## Gaps this journey surfaces

| # | Gap | Proposal (to decide) |
|---|---|---|
| G1 | **Kickoff:** nothing makes the lead start on the goal after a deploy; a TUI harness waits at its prompt | On deploy, Messaging delivers the goal to the lead as its first input ("Your goal on ledger: …"); the goal is also in the session prompt |
| G2 | **How the Chief of Staff runs:** which engine, which mounts, how it writes team files and `project.yaml` | An instance in a container whose only mounts are `.foreman/teams/` (rw) and clones of the projects' repos; it changes things only through its team tools, never `.foreman/` secrets or state |
| G3 | **Who writes cards:** the wiki is read-only except for the scrum master, so a lead can't create LED-44 when the team has one | Leads also get wiki write access; or leads ask the scrum master to write cards (slower, one more hop) |
| G4 | **Screen after a Foreman restart:** the server-side emulator lived in Foreman's memory, so a re-attached session has an empty screen until the program redraws | Replay the session log into a fresh emulator on re-attach (the log is complete; the emulator, unlike a browser, digests it from the start), or accept a blank screen until the next redraw |
| G5 | ~~**Git as a core block**~~ | **Resolved** by the CEO's decision to give git its own epic: Git is core block 8 in [Building blocks](./architecture/building-blocks.md#8-git-git), arriving with the [Git epic](../backlog/epics/git.md); until then agents share the mounted project folder and take turns |
