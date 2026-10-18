---
type: spec
status: planned
tags: [feature, architecture]
---

# The end-to-end journey

One continuous story through everything the UI reference allows, from installing Foreman to a merge into `main` and beyond. Each step says what the CEO does and sees, then **under the hood**: which [building blocks](./architecture/building-blocks.md) act, through which interface. A step no block can explain is a gap; gaps are collected at the end. The [use cases](./use-cases.md) are the acceptance checks for the same ground.

Cast: the CEO; the workspace `~/agents-ws`; project `ledger` (code repo `ledger-api`, already cloned there); team **Atlas** (Mara, lead; Tove and Ilya, engineers); later team **Vesper** (a challenge team).

## Part 1: set up

**1. Start Foreman.** `foreman ~/agents-ws`. The first time it asks `Create .foreman/ here? [y/N]`; then it prints the URL with the access token and opens the browser.
- *Under the hood:* Workspace & config (opens or creates `.foreman/`, the workspace repo: `git init` and its `.gitignore`; the access token); Web server (listens on 127.0.0.1:8888, access guard swaps the URL token for a cookie); Instance manager (re-attaches to agent containers still running from last time: none yet); Execution engine (Podman service started if needed: `podman system service` on Linux, `podman machine start` on macOS).

**2. Prepare the agent images.** Settings → Maintenance shows Base and Harnesses `not built yet`. The CEO presses **Refresh** (or the first deploy does it).
- *Under the hood:* Image builder (Podman): builds `localhost/foreman/base`, then `…/harness` (Claude Code, Pi…); workspace and project layers are transparent tags. Build logs stream to the page; the Event bus pushes the state changes.

**3. Log in to Claude Code.** Settings → Global → Profiles → `default` (Claude Code) → **Connect**, or the same button on Home's setup checklist. A dialog shows a terminal running `claude /login`; the CEO completes the browser sign-in. Once: every workspace's projects on `default` use it.
- *Under the hood:* a one-off **login container**: Execution engine starts it from the harness image with the profile's folder (`~/.local/share/foreman/profiles/default/`) mounted read-write; a Terminal streams it into the dialog exactly like any session. When it exits, Settings shows `connected`.

**4. Connect the forge.** Settings → Forge accounts → GitHub → **Connect**: host and token, checked against GitHub, never shown again.
- *Under the hood:* Forge adapter (validates the token); the secret store (`.foreman/secrets`, 0600, never mounted into containers).

**5. Add the project.** Sidebar → **+ New project** asks for a name, and lists below it the workspace folders that aren't projects yet, with their git repos. The CEO picks `ledger` → **Add project**.
- *Under the hood:* Workspace & config creates the project repo `ledger/project/`: `git init`, `project.toml` (repos found, `forge`), and a starter `wiki/` with `index.md` and `wiki.toml` (one Backlog board), as its first commit; `.sandboxes/` goes into the git excludes. The file watcher notices; the sidebar updates through the Event bus.

**6. Read the project.** Project → **Overview** shows the wiki's `index.md`; **Board** shows the Backlog board (empty); **Settings** shows `project.toml` read-only.
- *Under the hood:* Wiki block (the agentic-wiki engine reads the bundle `ledger/project/wiki/`; markdown rendered on the server; links resolved by the engine).

**7. Create a team.** Sidebar → **+ New team** opens the Chief of Staff drawer: "What is the new team for?" → "Build features for ledger" → a proposed roster (lead, two engineers) → **Create Atlas** → a review request; the CEO merges it.
- *Under the hood:* the Chief of Staff is an instance like any other (Instance manager, Harness adapter, Terminal, Execution engine) with **extra team tools** (`present_choices`, `show_panel`, `create_team`, `hire`…). The drawer renders its tool calls as chat; `create_team` is handled by Workflow, which writes `.foreman/teams/atlas.toml` on a branch of the workspace repo through Workspace & config and sends it back through `request_merge` as a review request. Once the CEO merges it (Merge to main), the file watcher picks it up and the sidebar shows Atlas. (Gap G2: how the Chief of Staff runs.)

**8. Tune an agent.** Team → Members → Tove → the Agent page: the CEO changes her look and personality. "Saved. Applies on every project."
- *Under the hood:* Workspace & config writes the team file (version-guarded against concurrent edits) and commits it: one commit per save in the workspace repo.

## Part 2: put the team to work

> **Phasing.** Every agent has its own worktree from epic 1 ([Terminal multiplexer](../backlog/epics/terminal-multiplexer.md)); the `switch`, `commit` and `merge` tools and hand-offs that name a branch come with epic 2 ([Messaging and hand-offs](../backlog/epics/messaging-handoffs.md)). Work items, checks and integration detection (16, 17) and the merge and PR in 19 rely on the [Git epic](../backlog/epics/git.md).

**9. Deploy.** Team → **Deploy to a project** → `ledger`, goal "Add idempotency keys to POST /settle" → **Deploy**.
- *Under the hood:* the deployment is stored (State store). Instance manager `Ensure(deployment)`, for each member:
  1. Git `Prepare`: a worktree of `ledger-api` at `ledger/.sandboxes/<agent>/ledger-api` (`git worktree add` on the host), on `wt/<agent>/main`; Mara's is on `lead/atlas/integration`.
  2. Image builder `Ensure(ledger)` → `localhost/foreman/ws/agents-ws/ledger`.
  3. Harness adapter `Launch`: the session prompt (soul + team + project + goal) as the instance's `CLAUDE.md`, the MCP config pointing at `foreman-agent`, the hooks config.
  4. Agent link: registers the instance's token.
  5. Execution engine `Start`: the container with its worktree (rw) and `ledger-api/.git` (read-only: agents read git, Foreman writes it) at their host paths, the live project repo `ledger/project/` at `<sandbox>/project` (rw for Mara, the lead, read-only for the others), its private config, the shared login file, `~/.gitconfig` (ro).
  6. Terminal: created on the process; status `starting` → `idle` on the first hook.
  7. **Kickoff:** only Mara, the lead, starts now: her harness starts with the goal as its opening message (`Your goal on ledger: …`), so she starts working at once. Tove and the other members are `asleep`: no container runs until they have work ([Agent runtime → Lazy starts](./architecture/agent-runtime.md#lazy-starts)).

**10. Watch.** Home: "4 sessions working across 1 deployment". **Sessions** → tab "Atlas · ledger" → Mara's pane (the lead's tab is first).
- *Under the hood:* Web server → Terminal `Attach` → a View: the snapshot of Mara's screen, then live output (WebSocket, raw bytes).

**11. Talk to the lead.** The CEO types into Mara's pane: "Use the existing request-id header; keep the API backwards compatible."
- *Under the hood:* keystrokes go raw, Web server → Terminal `Input` → the container's terminal. No Foreman logic in between.

**12. The lead plans in the wiki.** Mara reads `project/wiki/` and the code, creates the card `project/wiki/backlog/LED-44.md` (a task: status `todo`, assignee `tove`) and commits it with `commit()`. (The lead writes the knowledge base; everyone else contributes through `contribute`.)
- *Under the hood:* the agent edits files in the live project repo, mounted read-write for her (its `.git` read-only), and calls `commit()`, which Foreman runs on the host; Foreman's file watcher notices; the Wiki block re-reads; the Event bus updates the Board and the "changed" dot in the reader.

**13. The lead delegates.** Mara calls the team tool `ask(tove, "Take LED-44 …")`.
- *Under the hood:* the harness → `foreman-agent mcp` (stdio) → the agent port → Agent link (the token says `mara@ledger`) → Messaging: message `msg-41` queued for `tove@ledger`; `ask` returns `msg-41` at once, Mara keeps working. A **marker** "Mara asked Tove (msg-41)" appears beside both panes and in the Messages drawer (UI only).

**14. The engine wakes the teammate.** Tove is idle, so Messaging delivers: the text appears in Tove's terminal as if typed, and she starts.
- *Under the hood:* Instance manager knows Tove is `idle` (her last hook event); Messaging → `Instances.Deliver` → Harness adapter `Deliver` (types the text, then Enter) → Terminal `Input`. Tove's next hook says `busy`. Had she been busy, the message would wait for her next `idle`.

**15. The teammate works.** Tove reads `project/wiki/backlog/LED-44.md`, calls `switch("wt/tove/LED-44")`, which creates that branch from `main` and puts her worktree on it, writes the code and tests, runs `npm test` (the toolchain comes from the project's image layer), and calls `commit("Idempotency keys for /settle")`. On the way she finds a flaky test in another module and calls `contribute(finding, "Flaky settle test", …)`.
- *Under the hood:* the work is plain tools inside her container (git there is read-only: `status`, `diff`, `log`), which the CEO can watch live in her pane. `switch` and `commit` go through the Agent link to Workflow, and Git makes the branch and the commit on the host in her worktree, authored as Tove. `contribute` goes through the Agent link to Workflow, which writes `project/wiki/inbox/2026-10-14-tove-flaky-settle-test.md` (author from her token) and has Messaging deliver "⇄ Tove contributed: Flaky settle test (…)" to Mara at her next idle, with a marker.

**16. Hand-off.** Tove calls `handoff(mara, "wt/tove/LED-44", "Idempotency keys, tests pass")`.
- *Under the hood:* Workflow commits anything Tove left uncommitted, checks the branch against the policy and creates a work item (stage *handed to lead*). Nothing is fetched: Tove's and Mara's worktrees share the repo, so Mara sees the branch at once (agents never push anywhere, and nothing goes to the forge yet). Messaging delivers "Tove handed off wt/tove/LED-44: …" to Mara at her next idle; marker shown; the project's Repositories tab shows the item under In flight.

**17. Integration.** Mara reviews the branch (`git diff` in her worktree) and merges it into `lead/atlas/integration` with `merge("wt/tove/LED-44")`.
- *Under the hood:* `merge` → Workflow → Git merges the branch on the host, in Mara's worktree (a conflict would leave markers for her to resolve, then `commit`). Workflow notices the integration through Git (the branch is contained in a `lead/atlas/*` branch) when Mara next goes idle: the work item moves to *integrated*. (If she wanted changes instead: an `ask` back to Tove, which moves nothing; his next hand-off of the branch brings the work item back to her: [Protocol → Work items and merge requests](./architecture/protocol.md#work-items-and-merge-requests).)

**18. The merge request.** Mara calls `request_merge("lead/atlas/integration", "backlog/LED-44.md")`: the card explains what was done.
- *Under the hood:* Workflow checks the branch and the entry, runs the project checks (in a throwaway container from the project image), creates the merge request for `ledger-api` (titled from the card, carrying the integrated work items) and a **review request**. Nothing is pushed. Home shows it, the browser notifies, Mara receives "Merge requested".

**19. The CEO merges.** The review waits on Home and on the project's Overview, under Needs you. The CEO picks **Create PR on GitHub**, then **Open PR ↗**, reviews and merges on GitHub. (Or **Merge to main**, and Foreman merges locally; or **Reject** with a reason, which goes back to Mara.)
- *Under the hood:* Create PR: Git pushes the branch to GitHub from the host, with the workspace token, and the Forge adapter opens PR #128. Forge adapter polling then sees `merged`; Workflow resolves the request, marks the merge request and its work items merged, delivers "merged into main" to Mara, and tells the leads of other open merge requests on `ledger-api` to merge `main` in (`merge("main")`). Git fetches the new `main` into `ledger-api` (`Fetch`: remote-tracking refs only, never the CEO's checkout), where every worktree sees it. (With Merge to main, Git merges on the host instead, and nothing reaches GitHub.)

## Part 3: intervene

**20. A direct instruction.** In Sessions, the CEO presses **Split** and clicks Ilya's tab: his pane opens on the right with his current screen, banner "Direct line to Ilya. He'll tell Mara if it matters." The CEO types "Use the decimal library already in the repo."
- *Under the hood:* a second Terminal `Attach` (snapshot + live); keystrokes raw to Ilya's terminal. Ilya's prompt tells him to inform the lead if relevant, which he does with `ask`/`reply` like any message.

**21. An approval.** Ilya wants a new dependency, a project gate: he calls `escalate(approval, "Add decimal.js?", ["Approve", "Deny"])`. Home shows the request, the browser notifies. The CEO clicks **Approve**; Ilya continues.
- *Under the hood:* Agent link → Workflow: a request; **the tool call stays open**, so Ilya is paused; Event bus → Home and notification. `Resolve` returns "Approve" as the tool's result and Ilya resumes.

**22. The harness itself waits.** Pi stops at its own question ("Overwrite migrations? 1/2"). Home shows "Rhea is waiting in her session" with the choices; the CEO picks one, and it is typed into the session.
- *Under the hood:* the harness's hook → `foreman-agent hook` → Agent link → Instance manager; the Harness adapter's `Interpret` says *waiting for the user* (not just idle) → Workflow raises the request; the answer goes through `Deliver`.

**23. A crash.** Ilya's harness hits its rate limit and exits. Home: a **blocked** request, "Retry now · Retry in 30 min · Switch profile" (another account or harness for Ilya: later, [Profiles per agent](../backlog/profiles-per-agent.md)).
- *Under the hood:* Terminal `Done` → Instance manager → Harness adapter `Diagnose(exit code, last output)` → Workflow raises the blocked request; **Retry** makes the Instance manager start the instance again (same worktrees, same private config).

**24. A second team.** The CEO deploys **Vesper** (a challenge team) on `ledger` too. Kestrel finds a race and calls `escalate(finding, …, ["Atlas fixes it now", "Add to backlog", "Not an issue"])`. The CEO picks "Atlas fixes it now".
- *Under the hood:* two deployments on one project: separate instances, worktrees, lead branches (`lead/vesper/*`) and message channels (Messaging never crosses deployments). Workflow turns the answer into a delivery to Atlas's lead, Mara.

**25. Hire.** Team Atlas → **Hire** → the Chief of Staff interview → "Hire Arden". The CEO merges the review; Arden appears in the team and starts on every Atlas deployment.
- *Under the hood:* the Chief of Staff's `hire` tool → Workflow → the team file on a branch of the workspace repo → `request_merge` → a review request; once merged, the file watcher → Instance manager `Ensure` starts the new instance (as in step 9).

**26. Edit a soul while running.** The CEO edits Tove's job description. Running instances get a short context-update delivery at their next idle; the full prompt applies at their next start.
- *Under the hood:* Workspace & config (team file) → Instance manager → Messaging delivery; the Harness adapter regenerates `CLAUDE.md` for the next start.

**27. Change project config safely.** Project → Settings → **Change with Chief of Staff** → "Use GitHub as the forge". A review appears on the project's Overview, under Needs you.
- *Under the hood:* the Chief of Staff edits `project.toml` on a branch `foreman/config-1` of the project repo `ledger/project/` (never in the live checkout) → `request_merge` → a review request (Merge to main, Create PR on the project repo's forge, or Reject).

**28. Wiki and board.** In Overview, the CEO ticks a checkbox in a standup note; on the Board, drags LED-51 to "In progress".
- *Under the hood:* the Wiki block's version-guarded single-line writes (`SetCheckbox`, `SetField`); agents see the change at once in the live project repo they all mount.

## Part 4: stop and resume

**29. Stop a deployment.** Team Atlas → Deployments → **Stop** on ledger, after confirming. Its sessions end; worktrees are kept, and its agents' `wt/*` branches already merged into `main` are deleted; Vesper keeps running.
- *Under the hood:* Instance manager stops Atlas's instances (Execution engine `Stop`), revokes their tokens; its open requests expire; Messaging expires its queue; Git `Prune` deletes the merged `wt/*` branches.

**30. Restart Foreman.** The CEO quits and starts `foreman` again (an upgrade, say). Vesper's agents never stopped.
- *Under the hood:* the containers kept running; Instance manager `List`s them by label and re-attaches; their Terminals are rebuilt. (Gap G4: the server-side screen after a restart.)

## Gaps this journey surfaces

| # | Gap | Proposal (to decide) |
|---|---|---|
| G1 | ~~**Kickoff**~~ | **Resolved** (2026-10-15): only the lead starts on deploy, its harness launched with the goal as its opening message (still an interactive session); the other members sleep until they have work ([Agent runtime → Lazy starts](./architecture/agent-runtime.md#lazy-starts)) |
| G2 | **How the Chief of Staff runs:** which engine, which mounts, and where it makes its branches of the workspace repo and of the project repos | An instance in a container that works on its own branches of the workspace repo (team files) and of the project repos (`project.toml`); it changes things only through its team tools, never `.foreman/` secrets or state |
| G3 | ~~**Who writes cards**~~ | **Resolved** by the CEO's repo model: the lead writes the knowledge base (the live project repo), and commit through `commit()` (no review); everyone else contributes through `contribute` ([Sandboxes and git policy](./architecture/sandboxes-and-git.md#project-knowledge-base)) |
| G4 | ~~**Screen after a Foreman restart**~~ | **Resolved** (2026-10-14): a container still running is resumed (Foreman re-attaches and rebuilds the screen by replaying the session log into a fresh emulator); a container that is gone gets a new terminal session ([Agent runtime → Restarts](./architecture/agent-runtime.md#restarts)) |
| G5 | ~~**Git as a core block**~~ | **Resolved** by the CEO's decision to give git its own epic: Git is core block 8 in [Building blocks](./architecture/building-blocks.md#8-git-git), its `Prepare` (a worktree per agent) arriving in epic 1 and its merge plumbing with the [Git epic](../backlog/epics/git.md) |
