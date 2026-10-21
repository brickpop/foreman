---
type: spec
status: partial
tags: [architecture, api, ui, security]
---

# API

The HTTP surface the SPA uses: JSON over HTTP, server-sent events for push, and one WebSocket per terminal. Served by the same binary as the UI ([Overview](./overview.md)). Agents don't use this API: they use the [Team tools](./team-tools.md) tools on the separate team-tools listener.

All endpoints are local-only unless the listen setting changes. Every request passes the checks in [Security](./security.md#http-hardening) (Host allowlist, Origin check, access token).

## Endpoints

The PRD's surface (indicative; names may still change during implementation, this entry is then updated):

```
GET    /api/home                              {path}: Foreman's home on this machine
GET    /api/projects | /api/projects/:id      the record (name, profile, code repos with path and root commit), config (parsed), version, problems (`repos.<name>: …` for a missing or changed repo); deployments join with the Instance manager
POST   /api/projects                          {name, clone?} → New: the record and its project repo under the home, with a starter wiki; with clone (a git URL or a local path), its project repo cloned with the CEO's own git; or a project repo a removed project left there taken over (201 made or cloned, 200 taken over; 400 INVALID, with git's message for a failed clone; 409 EXISTS)
DELETE /api/projects/:id                      ?deleteRepo=true → remove the project: its record, sandboxes, session logs and history; its project repo folder only with deleteRepo (204; 409 DEPLOYED)
POST   /api/projects/:id/repos                {name, path, main?, new?} → add an existing code repo at path, on its main branch (main when it has one, else the `main` given, one of its branches), or with new, create an empty one on main at a path that doesn't exist yet (400 INVALID, 409 EXISTS)
PUT    /api/projects/:id/repos/:repo          {path} → Locate: the repo's new path, which must hold its root commit (400 INVALID)
PUT    /api/projects/:id/repos/:repo/name     {name} → rename it; agents' worktrees move, open merge requests follow (400 INVALID, 409 EXISTS, 409 DEPLOYED)
PUT    /api/projects/:id/repos/:repo/main     {main} → its main branch, one it has: agents start from it, merges go to it (400 INVALID)
DELETE /api/projects/:id/repos/:repo          remove it from the project; its folder and branches stay (409 DEPLOYED while a team is deployed on the project)
PUT    /api/projects/:id/profile              {profile} → the profile its agents run on; empty means default (400 INVALID)
GET    /api/projects/:id/repos                the code repos, then the project repo, each with its branches (ahead of main, uncommitted changes)
GET    /api/projects/:id/repos/:repo/tree     ?ref=&path= a folder at a ref, folders first
GET    /api/projects/:id/repos/:repo/file     ?ref=&path= a file at a ref: {path, size, text, binary, tooLarge} (over 1 MB: no text)
GET    /api/projects/:id/repos/:repo/log      ?ref=&skip= a page of history, newest first: {commits, next}, 50 commits; 404 NOT_FOUND for an unknown project, repo, ref or path
GET    /api/projects/:id/scaffold             the scaffold files the project repo lacks (`["wiki/index.md", "wiki/wiki.toml"]`); POST creates them, never overwriting, commits them as the CEO, and answers what is still missing
GET    /api/folders                           ?path= a folder for picking repos, inside the start folder (none: the start folder): {path, parent, entries: [{name, path, repo}]} (400 INVALID outside it)
GET    /api/folders/branches                  ?path= a repo folder's branches, to pick its main branch: {branches, current} (current: its checkout's, "" when detached; 400 INVALID outside the start folder)

GET    /api/teams | /api/teams/:id            definitions, with version and problems; `order` lists the agents in the file's order (the `agents` map loses it)| /api/teams/:id            definitions, with version and problems
PUT    /api/teams/:id/agents/:agent           {soul, version}: saves one soul in the team file (version-guarded) → the team

POST   /api/cos                              {opener}: starts the Chief of Staff's session, or types the opener into the running one → {terminal: "cos", proposal: the one on screen or null}; 409 NOT_READY (Podman), 409 NOT_CONNECTED (the default profile)
DELETE /api/cos                              stops it (the CEO left the page); the proposal goes with it
POST   /api/cos/proposal                     {id, looks}: writes proposal id, the one on screen, with the faces the page drew for members proposed without one; one commit → the team; 409 STALE (replaced, or none), 409 CONFLICT (the team changed since: the CoS is asked again), 400 INVALID

GET    /api/deployments[?status=stopped]      the active deployments, or the stopped ones (400 for another status)
POST   /api/deployments                       {team, project, goal?} (a blank goal is none: the CEO gives it in the lead's session)
DELETE /api/deployments/:id                   stop
GET    /api/instances                         the instances of the active deployments, with their status and activity line (doing)
GET    /api/instances/:id/prompt              {prompt}: the session prompt, exactly as composed for the harness
GET    /api/teams/:id/agents/:agent/prompt    {prompt}: the template before any deployment, {placeholders} for its parts
WS     /ws/pty/:instance                      bidirectional PTY bytes + resize
(line input: sent through the PTY WebSocket, as the same raw bytes typing would produce; no separate endpoint)
POST   /api/instances/:id/restart
POST   /api/instances/:id/wake            starts an asleep instance, or a stopped one of an active deployment (Start again); both pick up their conversation (204; any other status is left as it is)
POST   /api/instances/:id/restart         stops a running instance (starting, running, idle, awaiting) and starts it again on its project's profile as it is now, continuing its conversation unless the harness changed (204; any other status is left as it is; 404; 409 NOT_READY)
POST   /api/instances/:id/line            {text}: types the CEO's line into the session and sends it (204; 409 NOT_RUNNING)
PUT    /api/instances/:id/countdown       {paused}: pauses or resumes the countdown of messages waiting on the CEO's draft (204)

GET    /api/requests                          the open requests (today: review requests, one per review; Request → Phasing)
POST   /api/requests/:id/resolve              {answer}
GET    /api/messages?deployment=              message log
GET    /api/actions?deployment=               the git tools its agents called (switch, commit, merge, abort_merge, set_submodule, request_merge), oldest first
GET    /api/activity?project=|deployment=     [&before=<cursor>] a page of team traffic, messages and actions together, newest first: {items: [{message} | {action}], next}, 50 items; a project means its active deployments; 400 BAD_REQUEST for neither, both or a bad cursor
GET    /api/pipeline?project=                 work items and merge requests (open, and merged in the last 7 days)
GET    /api/reviews                           every review, newest first, with its merge requests and status, without comments (Review)
GET    /api/reviews/:id                       one review, with its comments where they are now ({now, outdated} each); 404
GET    /api/reviews/:id/diff                  [?since=1] [{repo, main, head, since, diff}] per repo open in it: against its main, or from the head at the last Request changes
POST   /api/reviews/:id/comments              {repo, path, side, line, commit, snippet, body} (no repo: general): a draft comment; 200 the comment; 400 INVALID; 409 NOT_OPEN
PUT    /api/reviews/:id/comments/:comment     {body}: edits a draft; 204; 409 SENT
DELETE /api/reviews/:id/comments/:comment     deletes a draft; 204; 409 SENT
POST   /api/reviews/:id/changes               {note}: Request changes: the drafts and the note to the lead in one message; 204; 400 INVALID (neither)
POST   /api/reviews/:id/merge                 {heads: {<repo>: <commit>}}: merges each repo named, at the commit the CEO read, its full hash (Pipeline and merges → Merge to main); 204; 409 NOT_MERGED says, per repo, why one didn't (a conflict: the lead is told to merge main in; the CEO's checkout in the way; the branch moved since that commit), the others merged; 409 NOT_OPEN
POST   /api/reviews/:id/close                 {reason}: closes it unmerged in every repo open, the reason to the lead, work items back to stage 0; 204; 409 NOT_OPEN
POST   /api/repos/global/sync                 Sync: fetch, merge in and push the global repo's main (stops on a conflict)
POST   /api/projects/:id/sync                 Sync: fetch, merge in and push the project repo's main (stops on a conflict)

GET    /api/wiki/:project/tree | /entry/:path | /boards     the tree; one entry: its Markdown and lookup tables (Wiki engine → Rendering; reading it marks it seen); every board with its cards
GET    /api/wiki/:project/image/:path         an image an entry links to, inside the wiki (Wiki engine → Rendering); 404 for anything else
PUT    /api/wiki/:project/checkbox/:path      {line, done, version}: 204; 409 CONFLICT when the entry changed, 400 INVALID when there's no checkbox on that line
PUT    /api/wiki/:project/card/:board/:path   {value, lane, version}: status to value, and lane when not empty; 204; 409 CONFLICT, 400 INVALID (no status, a lane on a board without lanes), 404 when the card isn't on that board

GET    /api/settings | PUT /api/settings       settings, version, problems; PUT sends {settings, version}: 409 on a stale version, 400 with the problems when invalid
| PUT /api/settings       settings, version, problems
GET    /api/harnesses                         the supported harnesses: their command, newline keystroke, and how a profile connects (login: a terminal; endpoint: a form)
GET    /api/preferences | PUT /api/preferences  the CEO's preferences, the same for every project: {theme, terminalFont, terminalFontSize} (theme: system, dark or light; terminalFont: jetbrains-mono, fira-code, source-code-pro or system; terminalFontSize: 10 to 24; 400 INVALID). A PUT changes the fields it names; the rest stay
GET    /api/profiles                          the CEO's profiles (each a harness and its credentials, shared by every project): name, harness, connected; default first
POST   /api/profiles                          {name, harness} → 201 the profile, with no login yet (400 INVALID: a bad name or an unsupported harness; 409 EXISTS)
DELETE /api/profiles/:name                    deletes it and its login, ending its login terminal (204; 400 INVALID for default; 404)
POST   /api/profiles/:name/connect            start its harness login in a terminal → {terminal}: open it with WS /ws/pty/:terminal (404; 400 INVALID for a harness set in a form, Pi)
DELETE /api/profiles/:name/connect            end its login, if one runs (the Connect dialog closed) → 204
GET    /api/profiles/:name/endpoint           a Pi profile's model server: {url, model, contextWindow, maxTokens, apiKeySet}, zero when unset; the key is never sent (400 INVALID for a login harness; 404)
PUT    /api/profiles/:name/endpoint           {url, apiKey, model, contextWindow, maxTokens} → 204; an empty apiKey keeps the one set (400 INVALID with the reason; 404)
POST   /api/profiles/:name/endpoint/models    {url, apiKey} → the ids the server lists (GET <url>/models, asked from Foreman; an empty apiKey uses the one set); 502 UNREACHABLE with the server's error (400 INVALID; 404)
GET    /api/secrets/forge                     [{kind, tokenSet}]: which forges have a token (never the token)
PUT    /api/secrets/forge/:kind               {url, token}: check the token against the forge, store it, save the host in forges.<kind>
GET    /api/maintenance                       {running, engine, podman, images}: the engine running now (podman or direct) and its state; why Podman can't be chosen ("" when installed); each image's name, ref, state, last build log
POST   /api/maintenance/rebuild               202: rebuild the images without cache; GET /api/maintenance follows it

GET    /api/setup                            the setup checklist: [{kind, subject, state, detail, log, machine?}] in order ([First run](../features/first-run.md))
POST   /api/setup/podman/start               202: start the stopped Podman machine (macOS); the checklist follows it
POST   /api/setup/podman/machine             {cpus, memory (GiB)}: 202, create and start the Podman machine (macOS) in the background; 409 CANT_CREATE when out of bounds or a machine exists
GET    /api/events                            SSE: requests, status, file changes
```


A project's Activity tab pages its traffic with `/api/activity` and lists its older sessions with `/api/deployments?status=stopped` ([Project activity](../features/project-activity.md)).
Its Repositories tab reads its repos read-only with `/api/projects/:id/repos…` ([Repositories](../features/repositories.md)).
The project routes answer with the project as it is now; a refusal is 400 INVALID, 409 EXISTS or 409 DEPLOYED with the reason. Removing a project deletes its history too ([Add a project → Removing a project](../features/add-project.md#removing-a-project)).
Resource ids: project = its name (unique in any case); team = team id; instance = `agent@project` (agent ids are unique across teams, see [Team definition](../models/team-definition.md#schema)); deployment = `team@project`.

## Error format

```json
{ "error": "Human readable message", "code": "MACHINE_CODE", "details": {} }
```

`details` is optional. Clients branch on `code`; messages never leak internals (paths outside the home, tokens, stack traces).

| Status | Meaning |
|---|---|
| 200 / 201 / 204 | Success / created / no content |
| 400 `VALIDATION_ERROR` | Malformed body or parameters |
| 403 `FORBIDDEN_ORIGIN` / 401 `UNAUTHORIZED` | Failed Host/Origin check or missing access token ([Security](./security.md)) |
| 404 `NOT_FOUND` | Unknown project, team, instance, entry…, or an endpoint the server doesn't have (never the web app's page) |
| 409 `VERSION_CONFLICT` | Guarded write against a stale version |
| 409 `ALREADY_DEPLOYED` | Deploying a team to a project it's already on |
| 400 `CANT_DEPLOY` | A deploy that can't go ahead: the team or the project has problems (a code repo missing or changed is one), or the same team is deployed on another project that shares a repo |
| 409 `NOT_READY` | Deploying or waking an agent before Podman runs and the instances are restored ([First run](../features/first-run.md)) |
| 500 `SERVER_ERROR` | Anything unexpected (logged with details server-side) |

## Version-guarded writes

Every file-backed object the UI can change (wiki entry, team definition, settings) is served with a `version` (a content hash). Writes send the version they were based on:
- matching version → write, respond with the new version;
- different version → `409 VERSION_CONFLICT` with the current version; the UI shows an error and reloads the object. Never last-writer-wins.

Wiki writes are carried out by the [wiki engine](./wiki-engine.md) (single-field frontmatter rewrite, single-character checkbox flip). A checkbox is identified by its source line, the way the engine keys checkboxes, and the version guard makes that safe: if the entry changed since the UI read it, the line may have moved, and the write is refused with `409`. (The PRD sent an `index`.)

## Events

`GET /api/events` is one SSE stream per browser tab. Each event is one `data:` line holding `{"type": …, "data": …}` (the Go `events.Event`), so the browser needs one handler for every type; the client refetches or patches its cache. A client that falls behind is cut off, and refetches on reconnect.

| Type | When | Payload |
|---|---|---|
| `request.created` / `request.resolved` | Workflow creates or resolves a request | the request |
| `instance.status` | an instance changes status or task | id, status, task |
| `instance.draft` | messages wait on the CEO's draft in an agent's terminal, each countdown step, and when they stop waiting | id, waiting (0: the notice goes), leftMs, counting, paused |
| `instance.doing` | an agent's harness reports a step of its work (a hook): its new activity line ([Harness adapters → Activity lines](./harness-adapters.md#activity-lines)) | id, doing (line, at) |
| `instance.marker` | Messaging or Workflow notes agent traffic for the CEO (shown beside the terminal, never written into it: [Protocol](./protocol.md#text-into-a-session-deliveries-and-markers)) | id, text, kind (`ask`, `reply`, `handoff`, `escalate`, `pr`, `note`) |
| `deployment.created` / `deployment.stopped` | | deployment |
| `team.message` | a message is routed | the message |
| `team.action` | an agent called a git tool | the action |
| `pipeline.changed` | a work item changes stage or checks, or a merge request is created, changes checks, gets a PR or changes state | the work item or merge request |
| `wiki.changed` | a project wiki's content changed (an entry, `wiki.toml`, or an image its entries link to); only on a real change, by content | `project`, `paths` (entries added, edited or removed, and `wiki.toml`; empty when only a linked image came or went), `versions` (each path's new version, `""` once gone) |
| `config.changed` | `project.toml`, a team file or settings changed (incl. parse errors) | `object` (`project`, `team`, `settings`), `id`, `version` (empty once the file is gone), `problems` |
| `projects.changed` | projects or their repos appeared, went or changed (a repo added, located, removed, gone missing) | `{}`: refetch the project list |
| `setup.changed` | Podman appeared, started, stopped or failed; the instances were restored; the background image build started or ended | `{}`: refetch the checklist |
| `cos.proposal` | the Chief of Staff proposed a team (checked), or the one on screen is gone (created, stale, the session ended) | the proposal (`id`, `team`, `current`: the team as it is, null for a new one), or null |

The notifier also derives browser notifications and toasts from these events on the client ([Notifications](../features/notifications.md)). On reconnect, the client refetches what it shows (SSE `Last-Event-ID` replay is not required).

## PTY WebSocket

`WS /ws/pty/:instance`:
- On connect, the server sends a **snapshot** of the current terminal state (screen, scrollback, cursor, modes; [Terminal state](./terminal-state.md)), then live output. An instance without a session is a `404`, before the upgrade.
- **Binary frames** carry raw terminal bytes, both ways (keystrokes in, output out).
- **Text frames** carry JSON control messages: `{"type":"resize","cols":120,"rows":40}` from the client; `{"type":"exit","code":…}` from the server (a `status` message is planned with instance statuses). Unknown control messages are ignored on both sides.
- Several clients may attach to one instance (two tabs, split panes); all receive output, any may type. The PTY size follows the most recent resize.
- A viewer that falls more than 1024 output chunks behind is dropped, so a slow tab never stalls the agent. The server closes it with code `1013` (try again later); the client reconnects at once, clears its screen and receives a fresh snapshot. Other disconnects are retried with a backoff from 250 ms up to 5 s. After the `exit` message the server closes normally (`1000`) and the client doesn't reconnect.
- The upgrade request passes the same Origin and token checks as the HTTP API: browsers do not apply the same-origin policy to WebSockets.

`POST /api/instances/:id/input` is the line-input alternative (mobile or paste): the text plus Enter is written to the PTY as the same raw bytes typing would produce. Foreman attaches no side effects to either path ([Sessions](../features/sessions.md#talking-to-the-team)).
