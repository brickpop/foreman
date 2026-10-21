---
type: spec
status: planned
tags: [feature, architecture]
---

# The end-to-end journey

One continuous story through everything the UI reference allows, from installing Foreman to a merge into `main` and beyond. Each step says what the CEO does and sees, then **under the hood**: which [building blocks](./architecture/building-blocks.md) act, through which interface. A step no block can explain is a gap; gaps are collected at the end. The [use cases](./use-cases.md) are the acceptance checks for the same ground.

Cast: the CEO; Foreman's home `~/.foreman`; project `ledger` (code repo `ledger-api`, already cloned at `~/dev/ledger-api`); team **Atlas** (Mara, lead; Tove and Ilya, engineers); later team **Vesper** (a challenge team).

## Part 1: set up

**1. Start Foreman.** `foreman`. It prints the URL with the access token and opens the browser (a second `foreman` would only print and open the same URL).
- *Under the hood:* Home & config (opens or creates the home `~/.foreman` and takes its lock; the global repo `config/`: `git init` and its `.gitignore`; the access token); Web server (listens on 127.0.0.1:8888, access guard swaps the URL token for a cookie); Instance manager (re-attaches to agent containers still running from last time: none yet); Execution engine (Podman service started if needed: `podman system service` on Linux, `podman machine start` on macOS).

**2. Prepare the agent images.** Settings → System → Maintenance shows Base and Harnesses `not built yet`. The CEO presses **Refresh** (or the first deploy does it).
- *Under the hood:* Image builder (Podman): builds `localhost/foreman/base`, then `…/harness` (Claude Code, Pi…); a project layer is a transparent tag. Build logs stream to the page; the Event bus pushes the state changes.

**3. Log in to Claude Code.** Settings → Agents → Profiles → `default` (Claude Code) → **Connect**, or the same button on the Dashboard's setup checklist. A dialog shows a terminal running `claude /login`; the CEO completes the browser sign-in. Once: every project on `default` uses it.
- *Under the hood:* a one-off **login container**: Execution engine starts it from the harness image with the profile's folder (`~/.foreman/profiles/default/`) mounted read-write; a Terminal streams it into the dialog exactly like any session. When it exits, Settings shows `connected`.

**4. Connect the forge.** Settings → Forge accounts → GitHub → **Connect**: host and token, checked against GitHub, never shown again.
- *Under the hood:* Forge adapter (validates the token); the secret store (`~/.foreman/secrets`, 0600, never mounted into containers).

**5. Add the project.** Sidebar → **+ New project** asks for a name, `ledger`, and a project repo: **New**. Then its repos: the CEO picks `~/dev/ledger-api` in the folder browser → **Add repo**, then **Done** ([Add a project](./features/add-project.md)).
- *Under the hood:* Home & config stores the project record (`POST /api/projects`) and creates the project repo `~/.foreman/projects/ledger/`: `git init`, `project.toml` (`forge`), `AGENTS.md` and a starter `wiki/` with `index.md` and `wiki.toml` (one Backlog board), as its first commit. Adding `ledger-api` (`POST /api/projects/ledger/repos`) checks it is a git repo with a commit, inside the CEO's home folder, and keeps its path and root commit on the record. The sidebar updates through the Event bus (`projects.changed`).

**6. Read the project.** Project → **Overview** shows the wiki's `index.md`; **Board** shows the Backlog board (empty); **Settings** shows `project.toml` read-only.
- *Under the hood:* Wiki block (the agentic-wiki engine reads the bundle `~/.foreman/projects/ledger/wiki/`; markdown rendered on the server; links resolved by the engine).

**7. Create a team.** Sidebar → **+ New team** opens the Chief of Staff page, its terminal full width. The CEO types "A team to build features for ledger", answers its questions, and the preview opens beside the terminal: Atlas, its playbook, a tab per member (lead, two engineers). **Adjust team** → "make Ilya a reviewer" → a new preview → **Create Atlas**.
- *Under the hood:* the Chief of Staff is an ad-hoc session like the login container (Harness adapter, Terminal, Execution engine), not an agent of a deployment, on the `default` profile, with `config/teams/` mounted read-only. Its one tool, `propose_team`, is checked by Home & config and pushed to the page (`cos.proposal`). **Create Atlas** makes Home & config write `config/teams/atlas.toml`, one commit in the global repo; the file watcher picks it up and the sidebar shows Atlas.

**8. Tune an agent.** Team → Members → Tove → the Agent page: the CEO changes her look and personality. "Saved. Applies on every project."
- *Under the hood:* Home & config writes the team file (version-guarded against concurrent edits) and commits it: one commit per save in the global repo.

## Part 2: put the team to work

> **Phasing.** Every agent has its own worktree from epic 1 ([Terminal multiplexer](../backlog/epics/terminal-multiplexer.md)); the `switch`, `commit` and `merge` tools and hand-offs that name a branch come with epic 2 ([Messaging and hand-offs](../backlog/epics/messaging-handoffs.md)). Work items, checks and integration detection (16, 17) and the merge and PR in 19 rely on the [Git epic](../backlog/epics/git.md).

**9. Deploy.** Team → **Deploy to a project** → `ledger`, goal "Add idempotency keys to POST /settle" → **Deploy**.
- *Under the hood:* the deployment is stored (State store). Instance manager `Ensure(deployment)`, for each member:
  1. Git `Prepare`: a worktree of `ledger-api` at `~/.foreman/sandboxes/ledger/<agent>/ledger-api` (`git worktree add` on the host), on `wt/<agent>/main`; Mara's is on `lead/atlas/integration`.
  2. Image builder `Ensure(ledger)` → the image ledger's agents run on (`localhost/foreman/harness` while every project shares one).
  3. Harness adapter `Launch`: the session prompt (soul + team + project + goal) as the instance's `CLAUDE.md`, the MCP config pointing at `foreman-agent`, the hooks config.
  4. Agent link: registers the instance's token.
  5. Execution engine `Start`: the container with its worktree (rw) and `ledger-api/.git` (read-only: agents read git, Foreman writes it) at their host paths, the live project repo `~/.foreman/projects/ledger/` at `<sandbox>/project` (rw for Mara, the lead, read-only for the others), its private config, the shared login file, `~/.gitconfig` (ro).
  6. Terminal: created on the process; status `starting` → `idle` on the first hook.
  7. **Kickoff:** only Mara, the lead, starts now: her harness starts with the goal as its opening message (`Your goal on ledger: …`), so she starts working at once. Tove and the other members are `asleep`: no container runs until they have work ([Agent runtime → Lazy starts](./architecture/agent-runtime.md#lazy-starts)).

**10. Watch.** Dashboard: "4 sessions working across 1 deployment". **Sessions** → tab "Atlas · ledger" → Mara's pane (the lead's tab is first).
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
- *Under the hood:* Workflow checks the branch and the entry, runs the project checks (in a throwaway container from the project image), creates the merge request for `ledger-api` (titled from the card, carrying the integrated work items) and a **review request**. Nothing is pushed. Dashboard shows it, the browser notifies, Mara receives "Merge requested".

**19. The CEO merges.** The review waits in Reviews, its count on the sidebar. The CEO opens it, reads the change file by file, and leaves two comments: **Request changes** sends them to Mara, who fixes them and asks again. The second time, the CEO picks **Merge to main**, and Foreman merges locally. (Or **Close** with a reason, which goes back to Mara; later, **Create PR on GitHub** to review and merge there.)
- *Under the hood:* Create PR: Git pushes the branch to GitHub from the host, with the CEO's GitHub token, and the Forge adapter opens PR #128. Forge adapter polling then sees `merged`; Workflow resolves the request, marks the merge request and its work items merged, delivers "merged into main" to Mara, and tells the leads of other open merge requests on `ledger-api` to merge `main` in (`merge("main")`). Git fetches the new `main` into `ledger-api` (`Fetch`: remote-tracking refs only, never the CEO's checkout), where every worktree sees it. (With Merge to main, Git merges on the host instead, and nothing reaches GitHub.)

## Part 3: intervene

**20. A direct instruction.** In Sessions, the CEO presses **Split** and clicks Ilya's tab: his pane opens on the right with his current screen, banner "Direct line to Ilya. He'll tell Mara if it matters." The CEO types "Use the decimal library already in the repo."
- *Under the hood:* a second Terminal `Attach` (snapshot + live); keystrokes raw to Ilya's terminal. Ilya's prompt tells him to inform the lead if relevant, which he does with `ask`/`reply` like any message.

**21. An approval.** Ilya wants a new dependency, a project gate: he calls `escalate(approval, "Add decimal.js?", ["Approve", "Deny"])`. Dashboard shows the request, the browser notifies. The CEO clicks **Approve**; Ilya continues.
- *Under the hood:* Agent link → Workflow: a request; **the tool call stays open**, so Ilya is paused; Event bus → Dashboard and notification. `Resolve` returns "Approve" as the tool's result and Ilya resumes.

**22. The harness itself waits.** Pi stops at its own question ("Overwrite migrations? 1/2"). Dashboard shows "Rhea is waiting in her session" with the choices; the CEO picks one, and it is typed into the session.
- *Under the hood:* the harness's hook → `foreman-agent hook` → Agent link → Instance manager; the Harness adapter's `Interpret` says *waiting for the user* (not just idle) → Workflow raises the request; the answer goes through `Deliver`.

**23. A crash.** Ilya's harness hits its rate limit and exits. Dashboard: a **blocked** request, "Retry now · Retry in 30 min · Switch profile" (another account or harness for Ilya: later, [Profiles per agent](../backlog/profiles-per-agent.md)).
- *Under the hood:* Terminal `Done` → Instance manager → Harness adapter `Diagnose(exit code, last output)` → Workflow raises the blocked request; **Retry** makes the Instance manager start the instance again (same worktrees, same private config).

**24. A second team.** The CEO deploys **Vesper** (a challenge team) on `ledger` too. Kestrel finds a race and calls `escalate(finding, …, ["Atlas fixes it now", "Add to backlog", "Not an issue"])`. The CEO picks "Atlas fixes it now".
- *Under the hood:* two deployments on one project: separate instances, worktrees, lead branches (`lead/vesper/*`) and message channels (Messaging never crosses deployments). Workflow turns the answer into a delivery to Atlas's lead, Mara.

**25. Hire.** Team Atlas → **Hire** → the Chief of Staff page, already asked to hire for Atlas → its questions → the preview, Arden's tab marked new → **Save Atlas**. Arden appears in the team and starts on every Atlas deployment.
- *Under the hood:* `propose_team` (Atlas with Arden) → the CEO's click → Home & config writes the team file, one commit → the file watcher → Instance manager `Ensure` starts the new instance (as in step 9).

**26. Edit a soul while running.** The CEO edits Tove's job description. Running instances get a short context-update delivery at their next idle; the full prompt applies at their next start.
- *Under the hood:* Home & config (team file) → Instance manager → Messaging delivery; the Harness adapter regenerates `CLAUDE.md` for the next start.

**27. Change project config safely.** *Later: how a Chief of Staff change to `project.toml` is approved is open ([Project config with the Chief of Staff](../backlog/cos-project-config.md)); below is the earlier proposal.* Project → Settings → **Change with Chief of Staff** → "Use GitHub as the forge". A review appears on the project's Overview, under Needs you.
- *Under the hood:* the Chief of Staff edits `project.toml` on a branch `foreman/config-1` of the project repo `~/.foreman/projects/ledger/` (never in the live checkout) → `request_merge` → a review request (Merge to main, Create PR on the project repo's forge, or Reject).

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
| G2 | ~~**How the Chief of Staff runs**~~ | **Resolved** (2026-10-18): an ad-hoc session like the login container, on the `default` profile, with its own folder and `config/teams/` read-only; it proposes with `propose_team` and Foreman writes on the CEO's click, one commit, no branch or review ([Chief of Staff → How it runs](./features/chief-of-staff.md#how-it-runs)) |
| G3 | ~~**Who writes cards**~~ | **Resolved** by the CEO's repo model: the lead writes the knowledge base (the live project repo), and commit through `commit()` (no review); everyone else contributes through `contribute` ([Sandboxes and git policy](./architecture/sandboxes-and-git.md#project-knowledge-base)) |
| G4 | ~~**Screen after a Foreman restart**~~ | **Resolved** (2026-10-14): a container still running is resumed (Foreman re-attaches and rebuilds the screen by replaying the session log into a fresh emulator); a container that is gone gets a new terminal session ([Agent runtime → Restarts](./architecture/agent-runtime.md#restarts)) |
| G5 | ~~**Git as a core block**~~ | **Resolved** by the CEO's decision to give git its own epic: Git is core block 8 in [Building blocks](./architecture/building-blocks.md#8-git-git), its `Prepare` (a worktree per agent) arriving in epic 1 and its merge plumbing with the [Git epic](../backlog/epics/git.md) |
